package push

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestJobSnapshotValidationAndLeaseSecrets(t *testing.T) {
	now := time.Now().UTC()
	base := JobSnapshot{ID: "job", SessionID: "session", DeviceID: "device", Purpose: "sync", TargetChangeSeq: 7, Provider: "fcm", RegistrationVersion: 1, SettingsVersion: 0, DeploymentID: "deployment", MessageID: "message", BotID: "bot", BotSeq: 1, SessionGeneration: 1, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if !base.valid() {
		t.Fatal("virtual settings version zero rejected")
	}
	for _, mutate := range []func(*JobSnapshot){func(j *JobSnapshot) { j.SettingsVersion = -1 }, func(j *JobSnapshot) { j.Purpose = "other" }, func(j *JobSnapshot) { j.Provider = "opush" }, func(j *JobSnapshot) { j.AlertEligible = true }, func(j *JobSnapshot) { j.RegistrationVersion = 0 }, func(j *JobSnapshot) { j.ExpiresAt = j.CreatedAt }, func(j *JobSnapshot) { j.CreatedAt = time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC) }} {
		bad := base
		mutate(&bad)
		if bad.valid() {
			t.Fatal("invalid snapshot accepted")
		}
	}
	alert := base
	alert.Purpose = "alert"
	alert.Provider = "opush"
	alert.AlertEligible = true
	if !alert.valid() {
		t.Fatal("alert snapshot rejected")
	}
	lease := JobLease{jobID: "job", until: now.Add(time.Minute)}
	copy(lease.secret[:], bytes.Repeat([]byte{0xa7}, 32))
	raw, _ := json.Marshal(lease)
	receipt, _ := json.Marshal(JobReservation{Lease: lease})
	formatted := fmt.Sprintf("%v %+v %#v %s %x %v", lease, lease, lease, lease, lease, JobReservation{Lease: lease})
	if string(raw) != "null" || strings.Contains(string(receipt), "Lease") || strings.Contains(formatted, hex.EncodeToString(lease.secret[:])) || strings.Contains(formatted, "167 167") {
		t.Fatal("lease secret serialized")
	}
}
