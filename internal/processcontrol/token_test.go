package processcontrol

import (
	"testing"
	"time"
)

func TestTargetTokenRoundTripAndTamper(t *testing.T) {
	issuer, err := NewTokenIssuer()
	if err != nil {
		t.Fatalf("issuer: %v", err)
	}
	created := time.Unix(0, 1712345678901234567)
	identity := ProcessIdentity{PID: 4242, Name: "test.exe", CreateTime: created}
	token, err := issuer.Issue(identity)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	got, err := issuer.Verify(token, 4242)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !got.Equal(created) {
		t.Errorf("creation time mismatch: %v vs %v", got, created)
	}
	if _, err := issuer.Verify(token, 4243); err == nil {
		t.Error("token must be bound to PID")
	}
	tampered := token[:len(token)-1] + "A"
	if _, err := issuer.Verify(tampered, 4242); err == nil {
		t.Error("tampered token accepted")
	}
	if _, err := issuer.Verify("garbage", 4242); err == nil {
		t.Error("garbage token accepted")
	}
}

func TestTargetTokenExpiry(t *testing.T) {
	issuer, err := NewTokenIssuer()
	if err != nil {
		t.Fatalf("issuer: %v", err)
	}
	now := time.Unix(1000, 0)
	issuer.now = func() time.Time { return now }
	token, err := issuer.Issue(ProcessIdentity{PID: 7, CreateTime: time.Unix(0, 123)})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := issuer.Verify(token, 7); err != nil {
		t.Fatalf("fresh token rejected: %v", err)
	}
	now = now.Add(tokenLifetime + time.Second)
	if _, err := issuer.Verify(token, 7); err == nil {
		t.Error("expired token accepted")
	}
}

func TestCreatePreviewRequiresVerifiedIdentity(t *testing.T) {
	store := NewOperationStore()
	if _, err := store.CreatePreview(10, ActionTerminate, PriorityNormal, ProcessIdentity{}); err == nil {
		t.Error("preview must require a target identity")
	}
	if _, err := store.CreatePreview(10, ActionTerminate, PriorityNormal, ProcessIdentity{PID: 10, Name: "lsass.exe", CreateTime: time.Now(), IsProtected: true}); err == nil {
		t.Error("preview must refuse protected targets")
	}
	if _, err := store.CreatePreview(10, ActionTerminate, PriorityNormal, ProcessIdentity{PID: 10, Name: "StackBroom.exe", CreateTime: time.Now()}); err == nil {
		t.Error("preview must refuse StackBroom")
	}
}
