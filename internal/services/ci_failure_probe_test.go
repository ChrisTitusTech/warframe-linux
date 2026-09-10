package services

import "testing"

func TestIntentionalCIFailureProbe(t *testing.T) {
	t.Fatal("F01b acceptance probe: intentional failure; remove before PR")
}
