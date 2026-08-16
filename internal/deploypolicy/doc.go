// Package deploypolicy checks a LabDNS desired-state document against
// the GitOps allowlists shipped with examples/labdns-deploy.
//
// These checks are repository policy, not v1alpha1 schema: they reject
// broadened client networks, unapproved upstreams, unsafe chaos caps,
// missing protected names, and mutable image tags. labdns verify
// --policies is the CLI surface so a copied deployment repo only needs
// the labdns binary.
package deploypolicy
