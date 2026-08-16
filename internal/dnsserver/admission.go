package dnsserver

import (
	"github.com/hilather/go-lab-dns/internal/dnswire"
	"github.com/hilather/go-lab-dns/internal/model"
)

type admitDecision struct {
	drop    bool
	rcode   model.RCode
	reason  string
	badVers bool
}

func admit(req *dnswire.Request, parseErr error, maxQuestions int) admitDecision {
	if req == nil {
		return admitDecision{drop: true, reason: "malformed"}
	}
	if req.QR {
		return admitDecision{drop: true, reason: "qr"}
	}
	if parseErr != nil {
		if !req.HeaderOK {
			return admitDecision{drop: true, reason: parseReason(parseErr)}
		}
		return admitDecision{rcode: model.RCodeFormErr, reason: "malformed"}
	}
	if req.Opcode != dnswire.OpcodeQuery {
		return admitDecision{rcode: model.RCodeNotImp, reason: "opcode"}
	}
	if req.QDCount == 0 {
		return admitDecision{rcode: model.RCodeFormErr, reason: "qdcount"}
	}
	if maxQuestions > 0 && req.QDCount > maxQuestions {
		return admitDecision{rcode: model.RCodeFormErr, reason: "qdcount"}
	}
	if req.Query.Name == "" {
		return admitDecision{rcode: model.RCodeFormErr, reason: "qname"}
	}
	if req.Query.Class != "" && req.Query.Class != model.ClassIN {
		return admitDecision{rcode: model.RCodeNotImp, reason: "class"}
	}
	if isZoneTransfer(req.Query.Type) {
		return admitDecision{rcode: model.RCodeNotImp, reason: "qtype"}
	}
	if req.HasEDNS && req.EDNS.Version != 0 {
		return admitDecision{rcode: model.RCodeFormErr, reason: "edns-version", badVers: true}
	}
	return admitDecision{reason: "ok"}
}

func parseReason(err error) string {
	switch err {
	case nil:
		return "ok"
	case dnswire.ErrEmpty:
		return "empty"
	case dnswire.ErrShortHeader:
		return "short"
	default:
		return "malformed"
	}
}

func isZoneTransfer(t model.RRType) bool {
	return t == "AXFR" || t == "IXFR"
}
