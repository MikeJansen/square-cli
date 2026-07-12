package main

import "testing"

func TestKebab(t *testing.T) {
	cases := map[string]string{
		"CreatePayment":           "create-payment",
		"ListPaymentRefunds":      "list-payment-refunds",
		"V1ListOrders":            "v1-list-orders",
		"ApplePay":                "apple-pay",
		"BookingCustomAttributes": "booking-custom-attributes",
		"Catalog":                 "catalog",
	}
	for in, want := range cases {
		if got := kebab(in); got != want {
			t.Errorf("kebab(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPascal(t *testing.T) {
	cases := map[string]string{
		"amount_money":  "AmountMoney",
		"source_id":     "SourceId",
		"CreatePayment": "CreatePayment",
		"id":            "Id",
	}
	for in, want := range cases {
		if got := pascal(in); got != want {
			t.Errorf("pascal(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFieldNameLeadingDigit(t *testing.T) {
	if got := fieldName("3d_secure"); got != "N3dSecure" {
		t.Errorf("fieldName = %q, want N3dSecure", got)
	}
}

func TestDocURL(t *testing.T) {
	want := "https://developer.squareup.com/reference/square/payments-api/create-payment"
	if got := docURL("Payments", "CreatePayment"); got != want {
		t.Errorf("docURL = %q, want %q", got, want)
	}
	// OAuth uses an override slug.
	if got := docURL("OAuth", "RevokeToken"); got != "https://developer.squareup.com/reference/square/oauth-api/revoke-token" {
		t.Errorf("docURL OAuth = %q", got)
	}
}

func TestNeedsPointer(t *testing.T) {
	yes := []string{"string", "int64", "Money", "bool"}
	no := []string{"[]string", "map[string]any", "json.RawMessage", "any"}
	for _, tp := range yes {
		if !needsPointer(tp) {
			t.Errorf("needsPointer(%q) = false, want true", tp)
		}
	}
	for _, tp := range no {
		if needsPointer(tp) {
			t.Errorf("needsPointer(%q) = true, want false", tp)
		}
	}
}
