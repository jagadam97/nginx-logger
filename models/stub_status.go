package models

// StubStatus is one sample from nginx's http_stub_status_module.
//
// Active/Reading/Writing/Waiting are instantaneous gauges. Accepts/Handled/
// Requests are monotonic counters that reset when nginx restarts, so rates are
// derived at query time rather than stored.
type StubStatus struct {
	Active   int64 `json:"active"`
	Accepts  int64 `json:"accepts"`
	Handled  int64 `json:"handled"`
	Requests int64 `json:"requests"`
	Reading  int64 `json:"reading"`
	Writing  int64 `json:"writing"`
	Waiting  int64 `json:"waiting"`
}
