package resolver

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

var (
	// ErrNilSnapshot is returned when Resolve is called without a snapshot.
	ErrNilSnapshot = errors.New("resolver: nil snapshot")
	// ErrUnknownZone is returned when the pre-selected zone ID is not in the index.
	ErrUnknownZone = errors.New("resolver: unknown zone id")
	// ErrInvalidZone is returned by Compile for fail-closed zone invariants.
	ErrInvalidZone = errors.New("resolver: invalid zone data")
)

// typeANY is the RFC 1035 * / ANY query type as presented by dnswire.
const typeANY model.RRType = "ANY"

// Resolve answers q from the pre-selected zone. It does not rediscover the
// zone (callers that need longest-suffix selection use ZoneIndex.Select).
//
// Flag rules for local data:
//   - AA is set only for authoritative positive and negative answers.
//     Overlay hits and fallthrough are never AA.
//   - AD is never set on local or synthesized data.
//   - CD is cleared on every local result (pass-through is a forwarder job).
//   - RA is left false on every local result.
//
// Zero Defaults.CNAMEDepth is not unlimited: it falls back to
// model.DefaultCNAMEDepth (8). Overlay CNAME chains that leave local data
// set Fallthrough and stop; the resolver never forwards.
func Resolve(ctx context.Context, snap *snapshot.Snapshot, q model.Query, zoneID model.ZoneID) (model.Result, error) {
	if err := ctx.Err(); err != nil {
		return model.Result{}, err
	}
	if snap == nil {
		return model.Result{}, ErrNilSnapshot
	}
	qname := canonicalOwner(string(q.Name), "")
	if qname == "" {
		qname = "."
	}
	if zoneID == "" {
		return fallthroughResult(snap, q, qname, "", ""), nil
	}
	zd, ok := snap.Zones.Lookup(zoneID)
	if !ok {
		return servfailResult(snap, q, qname, zoneID, ""), fmt.Errorf("%w: %s", ErrUnknownZone, zoneID)
	}
	if !zd.Contains(qname) {
		// Do not NXDOMAIN a name this zone does not own.
		return fallthroughResult(snap, q, qname, zd.ID, zd.Mode), nil
	}
	return resolveInZone(ctx, snap, q, qname, zd)
}

func resolveInZone(ctx context.Context, snap *snapshot.Snapshot, q model.Query, qname model.Name, zd *snapshot.ZoneData) (model.Result, error) {
	maxDepth := snap.Defaults.CNAMEDepth
	if maxDepth <= 0 {
		maxDepth = model.DefaultCNAMEDepth
	}

	var (
		answers []model.RR
		source  = model.SourceExact
		wildID  *model.RecordID
		encl    *model.Name
		current = qname
		seen    = map[model.Name]struct{}{}
		depth   int
	)

	for {
		if err := ctx.Err(); err != nil {
			return model.Result{}, err
		}
		if _, loop := seen[current]; loop {
			return servfailResult(snap, q, qname, zd.ID, zd.Mode), nil
		}
		seen[current] = struct{}{}

		if !zd.Contains(current) {
			if zd.Mode == model.ZoneModeOverlay {
				return overlayFallthrough(snap, q, qname, zd, answers, source, wildID, encl), nil
			}
			return positive(snap, q, qname, zd, answers, source, wildID, encl), nil
		}

		if zd.HasName(current) {
			res, next, done, err := exactOwner(snap, q, qname, zd, current, &answers, &source, &wildID, &encl, &depth, maxDepth)
			if err != nil || done {
				return res, err
			}
			current = next
			continue
		}

		res, next, done, err := wildcardOwner(snap, q, qname, zd, current, &answers, &source, &wildID, &encl, &depth, maxDepth)
		if err != nil || done {
			return res, err
		}
		current = next
	}
}

func exactOwner(
	snap *snapshot.Snapshot,
	q model.Query,
	qname model.Name,
	zd *snapshot.ZoneData,
	current model.Name,
	answers *[]model.RR,
	source *model.Source,
	wildID **model.RecordID,
	encl **model.Name,
	depth *int,
	maxDepth int,
) (model.Result, model.Name, bool, error) {
	qtype := q.Type

	if qtype == typeANY && len(*answers) == 0 {
		all := zd.AllRRsets(current)
		if soa := soaAsRRset(zd, snap, current); soa != nil {
			all = append([]snapshot.RRset{*soa}, all...)
		}
		if len(all) == 0 {
			return missExisting(snap, q, qname, zd, *answers, *source, *wildID, *encl), "", true, nil
		}
		*answers = append(*answers, rrsetsToRRs(all, current)...)
		return positive(snap, q, qname, zd, *answers, *source, *wildID, *encl), "", true, nil
	}

	if qtype == model.TypeSOA {
		if soa := soaAsRRset(zd, snap, current); soa != nil {
			*answers = append(*answers, rrsetToRRs(*soa, current)...)
			return positive(snap, q, qname, zd, *answers, *source, *wildID, *encl), "", true, nil
		}
	}

	if rr, ok := zd.RRset(current, qtype); ok {
		*answers = append(*answers, rrsetToRRs(rr, current)...)
		return positive(snap, q, qname, zd, *answers, *source, *wildID, *encl), "", true, nil
	}

	if qtype != model.TypeCNAME && qtype != typeANY {
		if rr, ok := zd.RRset(current, model.TypeCNAME); ok {
			*answers = append(*answers, rrsetToRRs(rr, current)...)
			return followCNAME(snap, q, qname, zd, rr, answers, source, wildID, encl, depth, maxDepth)
		}
	}

	return missExisting(snap, q, qname, zd, *answers, *source, *wildID, *encl), "", true, nil
}

func wildcardOwner(
	snap *snapshot.Snapshot,
	q model.Query,
	qname model.Name,
	zd *snapshot.ZoneData,
	current model.Name,
	answers *[]model.RR,
	source *model.Source,
	wildID **model.RecordID,
	encl **model.Name,
	depth *int,
	maxDepth int,
) (model.Result, model.Name, bool, error) {
	enc := zd.ClosestEncloser(current)
	wc := snapshot.WildcardOwner(enc)
	qtype := q.Type
	first := len(*answers) == 0

	if qtype == typeANY {
		all := zd.WildcardAll(wc)
		if len(all) == 0 {
			return missNonexistent(snap, q, qname, zd, *answers, *source, *wildID, *encl), "", true, nil
		}
		*source = model.SourceWildcard
		*wildID = ptrID(all[0].ID)
		*encl = ptrName(enc)
		*answers = append(*answers, rrsetsToRRs(all, current)...)
		return positive(snap, q, qname, zd, *answers, *source, *wildID, *encl), "", true, nil
	}

	if rr, ok := zd.Wildcard(wc, qtype); ok {
		*source = model.SourceWildcard
		*wildID = ptrID(rr.ID)
		*encl = ptrName(enc)
		*answers = append(*answers, rrsetToRRs(rr, current)...)
		return positive(snap, q, qname, zd, *answers, *source, *wildID, *encl), "", true, nil
	}

	if qtype != model.TypeCNAME && qtype != typeANY {
		if rr, ok := zd.Wildcard(wc, model.TypeCNAME); ok {
			*source = model.SourceWildcard
			*wildID = ptrID(rr.ID)
			*encl = ptrName(enc)
			synth := rr
			synth.Owner = current
			*answers = append(*answers, rrsetToRRs(synth, current)...)
			return followCNAME(snap, q, qname, zd, synth, answers, source, wildID, encl, depth, maxDepth)
		}
	}

	// RFC 4592: a source of synthesis that exists but lacks QTYPE is NODATA,
	// not NXDOMAIN. Overlay still fallthroughs (no local rule for this type).
	if first {
		*encl = ptrName(enc)
	}
	if zd.HasName(wc) || len(zd.WildcardAll(wc)) > 0 {
		if first {
			if all := zd.WildcardAll(wc); len(all) > 0 {
				*wildID = ptrID(all[0].ID)
			}
		}
		return missExisting(snap, q, qname, zd, *answers, *source, *wildID, *encl), "", true, nil
	}
	return missNonexistent(snap, q, qname, zd, *answers, *source, *wildID, *encl), "", true, nil
}

func followCNAME(
	snap *snapshot.Snapshot,
	q model.Query,
	qname model.Name,
	zd *snapshot.ZoneData,
	rr snapshot.RRset,
	answers *[]model.RR,
	source *model.Source,
	wildID **model.RecordID,
	encl **model.Name,
	depth *int,
	maxDepth int,
) (model.Result, model.Name, bool, error) {
	*depth++
	if *depth > maxDepth {
		return servfailResult(snap, q, qname, zd.ID, zd.Mode), "", true, nil
	}
	if len(rr.Data) == 0 {
		return missExisting(snap, q, qname, zd, *answers, *source, *wildID, *encl), "", true, nil
	}
	next := canonicalOwner(rr.Data[0], "")
	// Continue the loop at the CNAME target. Overlay-out-of-zone is
	// handled at the top of the next iteration via Contains.
	return model.Result{}, next, false, nil
}

func missExisting(
	snap *snapshot.Snapshot,
	q model.Query,
	qname model.Name,
	zd *snapshot.ZoneData,
	answers []model.RR,
	source model.Source,
	wildID *model.RecordID,
	encl *model.Name,
) model.Result {
	if len(answers) > 0 {
		if zd.Mode == model.ZoneModeOverlay {
			return overlayFallthrough(snap, q, qname, zd, answers, source, wildID, encl)
		}
		// In-zone CNAME target exists without QTYPE: keep CNAME, NODATA + SOA.
		return cnameNegative(snap, q, qname, zd, model.RCodeNoError, answers, source, wildID, encl)
	}
	if zd.Mode == model.ZoneModeOverlay {
		return fallthroughResult(snap, q, qname, zd.ID, zd.Mode)
	}
	return negative(snap, q, qname, zd, model.RCodeNoError, wildID, encl)
}

func missNonexistent(
	snap *snapshot.Snapshot,
	q model.Query,
	qname model.Name,
	zd *snapshot.ZoneData,
	answers []model.RR,
	source model.Source,
	wildID *model.RecordID,
	encl *model.Name,
) model.Result {
	if len(answers) > 0 {
		if zd.Mode == model.ZoneModeOverlay {
			return overlayFallthrough(snap, q, qname, zd, answers, source, wildID, encl)
		}
		// In-zone CNAME target does not exist: keep CNAME, NXDOMAIN + SOA.
		return cnameNegative(snap, q, qname, zd, model.RCodeNXDomain, answers, source, wildID, encl)
	}
	if zd.Mode == model.ZoneModeOverlay {
		return fallthroughResult(snap, q, qname, zd.ID, zd.Mode)
	}
	return negative(snap, q, qname, zd, model.RCodeNXDomain, wildID, encl)
}

func positive(
	snap *snapshot.Snapshot,
	q model.Query,
	qname model.Name,
	zd *snapshot.ZoneData,
	answers []model.RR,
	source model.Source,
	wildID *model.RecordID,
	encl *model.Name,
) model.Result {
	res := baseResult(snap, q, qname, zd.ID, zd.Mode)
	res.RCode = model.RCodeNoError
	res.Answers = answers
	res.Source = source
	res.WildcardSource = wildID
	res.ClosestEncloser = encl
	res.AA = zd.Mode == model.ZoneModeAuthoritative
	res.Explanation.Source = source
	res.Explanation.WildcardSource = wildID
	res.Explanation.ClosestEncloser = encl
	return res
}

// cnameNegative keeps the CNAME chain and applies the final in-zone name's
// RCODE plus SOA. Out-of-zone CNAME stops earlier (positive, no Fallthrough).
func cnameNegative(
	snap *snapshot.Snapshot,
	q model.Query,
	qname model.Name,
	zd *snapshot.ZoneData,
	rcode model.RCode,
	answers []model.RR,
	source model.Source,
	wildID *model.RecordID,
	encl *model.Name,
) model.Result {
	res := negative(snap, q, qname, zd, rcode, wildID, encl)
	res.Answers = answers
	res.Source = source
	res.Explanation.Source = source
	return res
}

func overlayFallthrough(
	snap *snapshot.Snapshot,
	q model.Query,
	qname model.Name,
	zd *snapshot.ZoneData,
	answers []model.RR,
	source model.Source,
	wildID *model.RecordID,
	encl *model.Name,
) model.Result {
	res := fallthroughResult(snap, q, qname, zd.ID, zd.Mode)
	res.Answers = answers
	if len(answers) > 0 {
		res.Source = source
		res.WildcardSource = wildID
		res.ClosestEncloser = encl
		res.Explanation.Source = source
		res.Explanation.WildcardSource = wildID
		res.Explanation.ClosestEncloser = encl
	}
	return res
}

func negative(
	snap *snapshot.Snapshot,
	q model.Query,
	qname model.Name,
	zd *snapshot.ZoneData,
	rcode model.RCode,
	wildID *model.RecordID,
	encl *model.Name,
) model.Result {
	res := baseResult(snap, q, qname, zd.ID, zd.Mode)
	res.RCode = rcode
	res.Source = model.SourceNegative
	res.AA = true
	res.WildcardSource = wildID
	res.ClosestEncloser = encl
	if soa := soaRR(zd, snap); soa != nil {
		res.Authority = []model.RR{*soa}
	}
	res.Explanation.Source = model.SourceNegative
	res.Explanation.WildcardSource = wildID
	res.Explanation.ClosestEncloser = encl
	return res
}

func fallthroughResult(snap *snapshot.Snapshot, q model.Query, qname model.Name, id model.ZoneID, mode model.ZoneMode) model.Result {
	res := baseResult(snap, q, qname, id, mode)
	res.RCode = model.RCodeNoError
	res.Source = model.SourceFallthrough
	res.Fallthrough = true
	res.AA = false
	res.Explanation.Source = model.SourceFallthrough
	return res
}

func servfailResult(snap *snapshot.Snapshot, q model.Query, qname model.Name, id model.ZoneID, mode model.ZoneMode) model.Result {
	res := baseResult(snap, q, qname, id, mode)
	res.RCode = model.RCodeServFail
	// SERVFAIL is not a local or negative answer; never set AA.
	res.AA = false
	return res
}

func baseResult(snap *snapshot.Snapshot, q model.Query, qname model.Name, id model.ZoneID, mode model.ZoneMode) model.Result {
	qq := q
	qq.Name = qname
	return model.Result{
		ZoneID:   id,
		ZoneMode: mode,
		// Local answers never forge AD and always clear CD.
		AD: false,
		CD: false,
		// RA is left false on every local result.
		RA: false,
		Explanation: &model.Explanation{
			Query:    qq,
			ZoneID:   id,
			ZoneMode: mode,
			Revision: snap.Revision,
		},
	}
}

func soaAsRRset(zd *snapshot.ZoneData, snap *snapshot.Snapshot, owner model.Name) *snapshot.RRset {
	if zd == nil || zd.SOA == nil || owner != zd.Name {
		return nil
	}
	rr := soaRR(zd, snap)
	if rr == nil {
		return nil
	}
	return &snapshot.RRset{
		Owner: owner,
		Type:  model.TypeSOA,
		Class: model.ClassIN,
		TTL:   rr.TTL,
		Data:  []string{rr.Data},
	}
}

func soaRR(zd *snapshot.ZoneData, snap *snapshot.Snapshot) *model.RR {
	if zd == nil || zd.SOA == nil {
		return nil
	}
	ttl := snap.Defaults.NegativeTTL
	if zd.SOA.Minimum > 0 && (ttl == 0 || zd.SOA.Minimum < ttl) {
		ttl = zd.SOA.Minimum
	}
	serial := zd.SOA.Serial
	if serial == "" || serial == "auto" {
		if snap.Generation == 0 {
			serial = "1"
		} else {
			serial = strconv.FormatUint(uint64(snap.Generation), 10)
		}
	}
	refresh := uint32(zd.SOA.Refresh / time.Second)
	retry := uint32(zd.SOA.Retry / time.Second)
	expire := uint32(zd.SOA.Expire / time.Second)
	minimum := uint32(zd.SOA.Minimum / time.Second)
	primary := string(zd.SOA.Primary)
	admin := string(zd.SOA.Administrator)
	data := fmt.Sprintf("%s %s %s %d %d %d %d", primary, admin, serial, refresh, retry, expire, minimum)
	return &model.RR{
		Name:  zd.Name,
		Type:  model.TypeSOA,
		Class: model.ClassIN,
		TTL:   ttl,
		Data:  data,
	}
}

func rrsetToRRs(rr snapshot.RRset, owner model.Name) []model.RR {
	if len(rr.Data) == 0 {
		return nil
	}
	class := rr.Class
	if class == "" {
		class = model.ClassIN
	}
	out := make([]model.RR, 0, len(rr.Data))
	for _, d := range rr.Data {
		out = append(out, model.RR{
			Name:  owner,
			Type:  rr.Type,
			Class: class,
			TTL:   rr.TTL,
			Data:  d,
		})
	}
	return out
}

func rrsetsToRRs(rrs []snapshot.RRset, owner model.Name) []model.RR {
	var out []model.RR
	for _, rr := range rrs {
		out = append(out, rrsetToRRs(rr, owner)...)
	}
	return out
}

func ptrID(id model.RecordID) *model.RecordID {
	if id == "" {
		return nil
	}
	v := id
	return &v
}

func ptrName(n model.Name) *model.Name {
	if n == "" {
		return nil
	}
	v := n
	return &v
}
