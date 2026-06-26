package services

import "testing"

func TestChannelName(t *testing.T) {
	got := ChannelName("Backend Engineer", "Jakarta Selatan")
	want := "backend-engineer-jakarta-selatan"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
