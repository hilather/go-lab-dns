# GA acceptance evidence (1.0.0-rc.1)

This index maps every criterion in [docs/19-acceptance-criteria.md](https://github.com/hilather/go-lab-dns/blob/main/docs/19-acceptance-criteria.md) to the tests and commands that prove it. It is the GA-001 evidence artifact. It is **not** a substitute for running the commands on the candidate commit.

1.1.0 operator-console evidence is the appendix **Operator console (1.1.0)** below. It does not replace the rc.1 tables.

**Candidate:** 1.0.0-rc.1 (untagged). A human creates the annotated tag only on an exact green commit. This change does not create a git tag or publish an image.

**Baseline:** first public candidate; public-surface diff is against the git empty tree.

**Date:** 2026-08-15

## How to re-run

```text
make format
make lint
make test
make test-race
make test-fuzz-smoke
make verify-generated
make test-docs
make test-parity
make test-config-compat
make test-integration
make test-changelog
make security-scan
make test-container          # requires Docker
go test ./benches -bench=. -benchmem
go run ./scripts/release-diff -notes-only -notes docs/releases/v1.0.0-rc.1.md
go run ./scripts/release-diff -print-empty-tree   # then:
make release-diff FROM=<empty-tree> TO=HEAD NOTES=docs/releases/v1.0.0-rc.1.md
```

Optional long soak (not the CI default): `go test ./internal/perf -soak=30m` or `LABDNS_SOAK_DURATION=30m`.

Local evidence on this branch (2026-08-15): `gofmt`; `go test ./...`; `make test`; `make test-docs`; `make test-integration`; `make verify-generated`; notes-only validation of [v1.0.0-rc.1.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/v1.0.0-rc.1.md). Required GitHub Actions jobs must be green on the **tag** commit before a human tags. There is no CI bypass. The 24h pre-GA soak was not run.

---

## Functional DNS

| Criterion | Evidence |
|---|---|
| Exact A, AAAA, CNAME, TXT, MX, SRV, PTR, CAA, NS, SOA, SVCB, HTTPS | `go test ./internal/resolver -run TestExactAllStructuredTypes`; `TestPackSampleFixture`; `internal/model.TestFirstGARRTypesLocked`; interop `testdata/interop/cases.json` exact-A |
| Wildcards: closest-encloser; exact names win | `go test ./internal/resolver -run 'TestRFC4592WildcardMatrix\|TestEmptyNonTerminalMatrix'`; `TestUDPTCPExactAndWildcard`; interop wildcard + empty-non-terminal rows |
| Authoritative NXDOMAIN vs NODATA include SOA | `go test ./internal/resolver -run TestAuthoritativeNXDOMAINVersusNODATA`; `TestUDPNXDOMAIN`; `go test ./internal/cache -run TestNXDOMAINVersusNODATA`; interop NXDOMAIN / NODATA AAAA |
| Overlay fallthrough only when no local answer | `TestOverlayFallthroughNeverNXDOMAIN`; `TestOverlayWildcardFallthroughOnWrongType`; `TestUDPOverlayMissIsRefusedOnWire`; `internal/dnsquery.TestOverlayCNAMEFallthroughForwardsTarget`; `internal/dnsquery.TestOverlayCNAMEForwardsWhenOriginalQNAMEHasNoPolicy`; `internal/dnsquery.TestUnknownClientLocalCacheMustNotSkipForward`; `internal/dnsquery.TestForwardThenUnknownThenForwardAgain` |
| UDP and TCP equivalent absent transport chaos | `go test ./internal/resolver -run 'TestUDPTCPExactAndWildcard\|TestUDPTCPFlagEquivalence'`; `internal/dnsserver` listener tests; interop UDP TC→TCP |
| Forwarding suffix selection and failover | `go test ./internal/forwarder -run 'TestCompileLongestSuffixAndDefaultDot\|TestFailoverMatrix\|TestUDPSuccessAndTCPFallback\|TestStrategiesOrderedRRRandom\|TestHealthAwareDeterministic'`; `internal/dnsquery.TestKnownGroupForwards`; `internal/perf.TestUpstreamOutageAndRecovery` |
| Positive and negative cache bounds | `go test ./internal/cache -run 'TestPositiveAndNegativeTTLClamp\|TestRevisionNamespace\|TestEvictionAndConcurrency\|TestDisabledAndZeroTTL'` |

Additional flag / refuse-forward matrix (pack `02`): `go test ./internal/dnsquery -run 'TestFlagMatrix\|TestUnmatchedForwardOnlyIsRefusedZeroPackets\|TestLocalOnlyGroupNeverForwards\|TestEmptyClientGroupsServesLocalForwardsNone\|TestAuthoritativeMissNeverForwards'`; `go test ./internal/resolver -run TestLocalFlagMatrix`.

---

## Chaos

| Criterion | Evidence |
|---|---|
| Fixed or uniform delay on exact or wildcard RRset | `go test ./internal/chaos/effects -run 'TestSleepFixedAndCancelReleasesBudget\|TestSleepFiresOnAdvance'`; `go test ./internal/chaos -run TestDecideRecordAndGlobal`; `internal/perf.TestMaxDelayedConcurrency` |
| Delay cancellable and globally bounded | `TestSleepFixedAndCancelReleasesBudget`; `TestConcurrentDelayBudget`; `TestEmergencyCancelAllUnblocksDelays`; `TestDelayClampAndSimulateNoBudget`; `TestBudgetExhausted`; `internal/perf.TestDelayBudgetReleasedAfterCancel` |
| Deterministic seeded decisions reproduce (`hash-v1`) | `go test ./internal/chaos -run 'TestHashV1BitForBitGoldens\|TestHashV1StableAcrossRestarts\|TestHashV1TimeBucketSameSecondAndNext'`; goldens [testdata/hash-v1/vectors.json](https://github.com/hilather/go-lab-dns/blob/main/testdata/hash-v1/vectors.json) |
| RCODE, NODATA, drop, TC, TCP close/reset, TTL, alternate, partial, ordering, flap, cache, upstream, rate | `go test ./internal/chaos/effects`; `TestRCodeNODATAAndErrors`; `TestDropWinsOverRCodeOnWire`; `TestHintMapping`; `TestTTLBoundariesAndNoOverflow`; `TestAlternateAllowlistAndCNAMELoop`; `TestPartialAnswerImmutability`; `TestCacheHooks`; `TestExchangeOpts`; `TestPressureRejectsOverCap`; `TestStartExpiryFlapEveryNth`; catalog [api/chaos/effects.json](https://github.com/hilather/go-lab-dns/blob/main/api/chaos/effects.json) |
| Protected names and clients not affected | `go test ./internal/chaos -run TestProtectedNameAndExemptGroup`; `internal/auth.TestProtectedObjectsOrdinaryRoles` |
| Emergency disable works during load | `go test ./internal/perf -run TestEmergencyDisableUnderLoad`; `internal/chaos.TestEmergencyDisableStampsStore`; `TestServeSignalsUSR1`; `cmd/labdns.TestSIGUSR1WorksWithManagementUnbound`; `internal/control/rest.TestPacketChaosThenRESTEmergency` |
| Simulation never mutates live state or sleeps | `go test ./internal/chaos -run 'TestSimulateDoesNotSleep\|TestDelayClampAndSimulateNoBudget\|TestSimulateEvaluatesDisabledPolicy'` |

Three independent emergency controls: startup `--chaos-disable` / `LABDNS_CHAOS_DISABLE` (`cmd/labdns.TestStartupChaosDisableWinsOverYAML`); authenticated `dns_chaos_emergency_disable` (`internal/control/rest.TestChaosEmergencyRoute`); `SIGUSR1` / `labdns chaos emergency-disable --pid-file` (`TestSIGUSR1WorksWithManagementUnbound`).

---

## State

| Criterion | Evidence |
|---|---|
| Startup loads strict YAML | `cmd/labdns.TestServeFromConfigPackSampleNS1`; `internal/config.TestDecodePackSampleYAML`; `internal/compiler.TestCompilePackSampleAccessAndRevision`; invalid bootstrap does not listen (`TestServeFromConfigInvalidDoesNotListen`) |
| Unknown fields fail | `go test ./internal/config -run 'TestDecodeJSONUnknownField\|TestDecodeUnknownFieldEveryLevel'`; `make test-config-compat` (negative fixtures under `testdata/config`) |
| Mutations use expected revision and idempotency | `go test ./internal/app -run 'TestRevisionConflict\|TestMissingExpectedRevisionFailClosed\|TestIdempotencySameKeySameBody\|TestIdempotencySameKeyDifferentBody\|TestIdempotencyPlanThenApplySameKey'` |
| Candidate atomically swapped | `TestPlanDoesNotSwapApplyDoes`; `TestInvalidOperationLeavesActiveUnchanged`; `TestInvalidCandidateCompileLeavesActiveUnchanged`; `internal/dnsquery.TestChaosRaceDecideAndSwap`; `internal/perf.TestSoakSwapsAndExpiry` |
| Reset safely reloads bootstrap | `go test ./internal/app -run 'TestResetRestoresBootstrap\|TestFailedResetKeepsIdempotencyCache'`; `cmd/labdns.TestRestartDiscardsRuntimeDrift`; `examples/labdns-deploy.TestRecreationResetsRuntimeDrift` |
| Runtime drift is visible | `internal/app.TestStatusDriftWarning`; `TestResetRestoresBootstrap` (`Drifted`); Status DTO `revisions` |
| Canonical export and deployment operations deterministic | `internal/app.TestExportDeterministic`; `TestExportBootstrapToRuntimeOps`; `internal/config.TestRevisionStableAcrossFormatting`; `TestCanonicalExportMaterializesDefaults`; `examples/labdns-deploy.TestEnvironmentsValidateAndVerify` |

---

## REST and MCP

| Criterion | Evidence |
|---|---|
| Every public capability has parity | `go test ./internal/capabilities -run 'TestParityHarnessMatchesDesignTable\|TestParityWritesHaveMCPTools\|TestParityMCPMutationsHaveREST'`; `go test ./internal/control/mcp -run 'TestParityEveryMutatingRESTHasMCPTool\|TestParityGoldens\|TestStructuredDTOMatchesREST'`; `make test-parity` |
| REST contract and MCP conformance | `go test ./internal/control/rest -run 'TestRoutesRegisteredFromRegistry\|TestContractReads\|TestContractMutations\|TestGeneratedOpenAPIMatchesRender'`; `go test ./internal/control/mcp -run 'TestToolsRegisteredFromRegistry\|TestResourcesRegisteredFromRegistry\|TestContractReads\|TestContractMutations\|TestPinnedProtocolDiscover'` |
| Shared authorization and errors | `internal/auth.TestRoleCapabilityMatrix`; `internal/control/rest.TestRESTRoleMatrixSharedWithAuth`; `TestRESTSharedAuthorizerMatchesMCP`; `internal/control/mcp.TestMCPRoleMatrix`; `internal/capabilities.TestProblemAndJSONRPCShareDomainData`; `internal/control/mcp.TestParityStructuredErrorsMatch` |
| MCP Streamable HTTP Origin and protocol version | `go test ./internal/control/mcp -run 'TestProtocolVersionRequired\|TestProtocolVersionMismatch\|TestDiscoverAdvertisesOnlyPinnedVersion\|TestOriginRemoteDenied\|TestOriginAllowlist\|TestStreamableHTTPOnlyPOST'` |
| Mutations support planning and audit | REST/MCP `TestContractMutations`; `TestIdempotentApplySameKey`; `internal/app` plan/apply tests; `internal/audit.TestRingBoundsAndOrder`; `internal/control/rest.TestRESTDeniedAuthorizationInAppAudit` |

---

## Security

| Criterion | Evidence |
|---|---|
| Not an open resolver by default | `internal/dnsquery.TestUnmatchedForwardOnlyIsRefusedZeroPackets`; `TestEmptyClientGroupsServesLocalForwardsNone`; `TestLocalOnlyGroupNeverForwards`; `cmd/labdns.TestVerifyGitOpsTemplateAndUnknownClient`; `internal/config.TestValidateUnknownClientClosed` |
| Management isolated and authenticated | `internal/auth.TestIdentifyRemoteRequiresBearer`; `TestIdentifyLoopback`; `internal/control/rest.TestRemoteUnauthenticatedDenied`; `TestRemoteXForwardedForNotTrusted`; `TestRESTOriginDeniedAndCORS`; `cmd/labdns.TestServeTemplateBearerRequiresSecret`; `examples/labdns-deploy.TestComposeAndK8sIsolateManagementAndPinDigest` |
| Container non-root, read-only, capability-free, scanned | `cmd/labdns.TestDockerfileHardening`; `make test-container` (`scripts/test-container.sh`); `make security-scan` (`govulncheck`) |
| Chaos separate scopes, expiry, caps, emergency | `internal/auth.TestChaosPrivilegeSeparation`; `internal/config.TestValidateHighImpactRequiresExpiry`; `internal/chaos.TestCompileHighImpactCap`; emergency tests above |
| No secret in export, logs, or public errors | `internal/auth.TestRedactJSON`; `TestRedactYAML`; `TestRedactOperationsAndState`; `internal/audit.TestRedactEventSecrets`; `internal/observability.TestLoggerRedactsQNAMEAndClient`; `TestRegistryRejectsQNAMEAndClientIP`; `examples/labdns-deploy.TestNoSecretsInTree` |

Reporting path: [SECURITY.md](https://github.com/hilather/go-lab-dns/blob/main/SECURITY.md) → GitHub private advisories on `hilather/go-lab-dns`.

---

## Quality

| Criterion | Evidence |
|---|---|
| Every area has regression tests | `go test ./...` (packages: model, config, resolver, forwarder, cache, compiler, snapshot, app, chaos, dnsquery, dnsserver, dnswire, rest, mcp, auth, audit, observability, capabilities, interop, perf, deploypolicy, examples, cmd, scripts) |
| Race, fuzz smoke, integration, parity, container, docs, security CI | Required jobs in [docs/14-release-engineering.md](https://github.com/hilather/go-lab-dns/blob/main/docs/14-release-engineering.md): `format lint unit race fuzz-smoke generated-file documentation security-scan container-test changelog parity config-compat`. Local recorded on this SHA: `make test` / `test-docs` / `test-integration` / `verify-generated`. `make test-race`, `test-fuzz-smoke`, `test-container`, and `security-scan` remain for the tag-gate matrix. Race-sensitive unit tests exist: `internal/resolver.TestResolveConcurrentAndImmutable`; `internal/dnsquery.TestChaosRaceDecideAndSwap`; `internal/testutil.TestFakeClockConcurrentAdvanceAndNow` |
| Load and soak targets met and documented | Short CI soak only: `make test-integration`; `go test ./internal/perf`; `go test ./benches`. Methodology [docs/10-testing-strategy.md](https://github.com/hilather/go-lab-dns/blob/main/docs/10-testing-strategy.md); capacity [docs/11-deployment.md](https://github.com/hilather/go-lab-dns/blob/main/docs/11-deployment.md). Absolute QPS is recorded, not CI-gated. The 24h pre-GA soak was **not** run — see [docs/known-limitations.md](https://github.com/hilather/go-lab-dns/blob/main/docs/known-limitations.md) |
| No known critical/high vuln without governance | `make security-scan` (govulncheck) is the local command; it was **not** recorded on this SHA. No accepted high/critical at candidate time. Tag-time SBOM/provenance is an operator step ([docs/14-release-engineering.md](https://github.com/hilather/go-lab-dns/blob/main/docs/14-release-engineering.md)) |
| Documentation matches implementation | `make test-docs`; `make verify-generated`; capability/docs table tests (`TestDocs05TableCoversFrozenTitles`, `TestDocs07ListsEveryResource`, `TestErrorMapMatchesDocs17Tables`); this index |

---

## Release

| Criterion | Evidence |
|---|---|
| Release notes include all functionality differences from the previous tag | First candidate: notes [v1.0.0-rc.1.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/v1.0.0-rc.1.md) vs empty tree. `go run ./scripts/release-diff -notes-only -notes docs/releases/v1.0.0-rc.1.md`; after a clean commit, `make release-diff FROM=<empty-tree> TO=HEAD NOTES=docs/releases/v1.0.0-rc.1.md` |
| API, MCP, config, metrics, CLI, defaults, chaos action diffs reviewed | Public surfaces in `internal/releasecontract.PublicSurfaces`; checklist in the notes file (all boxes checked for first-GA introduction) |
| All CI passes on the tagged commit | **Not yet.** No tag in this change. `tag-gate` in `.github/workflows/release.yml` refuses a tag unless required jobs succeeded on that SHA (`release-diff -require-ci`) |
| Previous CI failure has a documented fix and hardening | [docs/ci-failure-hardening/2026-08-15-cli-help-not-generated.md](https://github.com/hilather/go-lab-dns/blob/main/docs/ci-failure-hardening/2026-08-15-cli-help-not-generated.md); `scripts/checktargets.TestWorkflowsForbidBroadRetries` |
| Deployment and rollback tested | `examples/labdns-deploy.TestRollbackRestoresPriorDesiredState`; `TestRecreationResetsRuntimeDrift`; `TestScriptsFailClosedAndNoBypass`; `TestComposeAndK8sIsolateManagementAndPinDigest`; runbooks [docs/13-operations-and-runbooks.md](https://github.com/hilather/go-lab-dns/blob/main/docs/13-operations-and-runbooks.md) |

---

## Runbooks exercised (automated stand-in)

A dedicated staging-lab operator drill is a **human tag-time** step. The following tests execute the same actions the runbooks prescribe:

| Runbook | Automated exercise |
|---|---|
| Unexpected resolution | `dns_explain_resolution` / `resolve:explain` in REST/MCP contract tests; `labdns verify` (`cmd/labdns.TestVerifyProbes`) |
| Excessive DNS latency / chaos runaway | `internal/perf.TestEmergencyDisableUnderLoad`; `TestMaxDelayedConcurrency`; SIGUSR1 + startup lock tests |
| Reset to bootstrap | `internal/app.TestResetRestoresBootstrap`; `cmd/labdns.TestRestartDiscardsRuntimeDrift` |
| Promotion / rollback | `examples/labdns-deploy.TestRollbackRestoresPriorDesiredState`; `scripts/rollback.sh` fail-closed |
| Disaster recovery from deploy repo + secrets only | ADR 0003 + `TestNoSecretsInTree` + reset/recreation tests (runtime is ephemeral) |
| Fresh-machine template | `examples/labdns-deploy.TestEnvironmentsValidateAndVerify`; `TestTestConfigScriptPositivesAndNegatives` |

---

## Architectural invariants reviewed

Reviewed against [docs/01-architecture.md](https://github.com/hilather/go-lab-dns/blob/main/docs/01-architecture.md) and ADRs 0001–0007 on 2026-08-15:

| Invariant | Holds via |
|---|---|
| REST/MCP are adapters; one application layer | `internal/control/rest.TestHandlersCallServiceOnly`; `TestNoMutationPrimitivesInProduction`; MCP stdio uses the same registry |
| DNS serves an immutable compiled snapshot | `compiler.Compile` + `snapshot.Store`; soak asserts no mixed answers |
| No write to bootstrap file | `internal/app.TestApplyDoesNotWriteBootstrapFile` |
| No host-resolver fallback | `internal/config.TestValidateTransportClosedNoDoT`; no resolv.conf key in v1alpha1 |
| Unknown clients never forwarded (RA=0) | dnsquery refuse-forward tests |
| Chaos cannot affect REST/MCP/ready/live/emergency | `internal/observability.TestChaosDoesNotAffectHealth`; `internal/app.TestStatusChaosDoesNotUnready`; `internal/control/rest.TestPacketChaosIndependentOfREST` |
| Malformed-wire generation not in process | ADR 0007; `internal/chaos/effects` doc; catalog residual |
| MCP pinned to 2026-07-28 | `internal/buildinfo` + MCP protocol tests |
| Required CI has no bypass | `scripts/checktargets`; release `tag-gate` |

Open questions: first-GA decisions are frozen (Apache-2.0, module `github.com/hilather/go-lab-dns`, Go 1.26, image `ghcr.io/hilather/labdns`, MCP 2026-07-28 only, SIGUSR1 as emergency #3, overlay CNAME may terminate in a forwarded name). Remaining pack items are explicit follow-ons in [docs/known-limitations.md](https://github.com/hilather/go-lab-dns/blob/main/docs/known-limitations.md) and [docs/18-roadmap-and-non-goals.md](https://github.com/hilather/go-lab-dns/blob/main/docs/18-roadmap-and-non-goals.md).

---

## Program board

All work packages FND-001 through GA-001 are **done** on the 1.0.0-rc.1 candidate. UI-001 through UI-004 (M6) are **done** on the 1.1.0 increment. See [tasks/00-program-board.md](https://github.com/hilather/go-lab-dns/blob/main/tasks/00-program-board.md). M5/M6 *tag* remains a human step on a green commit.

---

## Operator console (1.1.0)

Evidence for the REST/MCP/UI rows added in [docs/19-acceptance-criteria.md](https://github.com/hilather/go-lab-dns/blob/main/docs/19-acceptance-criteria.md). Normative UI spec: [docs/22-web-ui.md](https://github.com/hilather/go-lab-dns/blob/main/docs/22-web-ui.md). Candidate notes: [docs/releases/v1.1.0.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/v1.1.0.md).

| Criterion | Evidence |
|---|---|
| Every `PARITY_REQUIRED` capability completable in the UI | `go test ./internal/capabilities -run TestParityUIBindingsComplete`; Playwright `web/e2e/operator.spec.ts`; capability map in [docs/22-web-ui.md](https://github.com/hilather/go-lab-dns/blob/main/docs/22-web-ui.md) |
| UI does not skip plan/apply | `web/src/pages/changes/ChangesPage.test.tsx`; Playwright plan then apply a record; REST still authorizes |
| Bearer/CSRF not in Web Storage | `web/src` session-memory Vitest; Playwright `expectNoSecretStorage`; `web/e2e/a11y.spec.ts` |
| Session/CSRF and Origin | `go test ./internal/auth ./internal/control/rest` session tests; cookie-present POST does not Identify |
| `ui.enabled: false` 404s SPA | REST tests for pre-auth `GET /` 404; `/v1/state` still authenticates |
| Chaos cannot affect UI/session/REST/MCP/health/emergency | Management-only prefix list; chaos packages do not import `internal/web` |
| Playwright operator matrix | `make web-e2e` / CI job `web` (`npx playwright install --with-deps chromium`; `npm run test:e2e`); fixture [testdata/web/](https://github.com/hilather/go-lab-dns/blob/main/testdata/web/config.yaml) |
| Required CI `web` | `internal/releasecontract.RequiredCIJobs`; `make web-test`; `make web-build` |

Re-run: `make web-test`, `make web-build`, `make web-e2e`, `make test-parity`, `make test-docs`, `go run ./scripts/release-diff v1.0.0-rc.2 HEAD -notes docs/releases/v1.1.0.md`.
