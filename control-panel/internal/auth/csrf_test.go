package auth

import (
	"testing"
	"time"
)

func TestCSRFBindsToSession(t *testing.T) {
	svc := NewService([]byte("s3cret"), time.Hour)
	jwt, err := svc.Issue("alice")
	if err != nil {
		t.Fatal(err)
	}
	csrf := svc.IssueCSRF(jwt)
	if !svc.VerifyCSRF(jwt, csrf) {
		t.Fatal("fresh token must verify")
	}
	if svc.VerifyCSRF(jwt, csrf[:len(csrf)-2]+"00") {
		t.Fatal("tampered token must not verify")
	}
	other, _ := svc.Issue("mallory")
	if svc.VerifyCSRF(other, csrf) {
		t.Fatal("token bound to another session must not verify")
	}
	foreign := NewService([]byte("other-secret"), time.Hour)
	if foreign.VerifyCSRF(jwt, csrf) {
		t.Fatal("token must not verify under a different secret")
	}
	if svc.VerifyCSRF(jwt, "") || svc.VerifyCSRF(jwt, "zzz") {
		t.Fatal("empty/garbage token must not verify")
	}
}
