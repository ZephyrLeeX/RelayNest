package auth

import (
	"strings"
	"testing"
)

func TestPasswordHash(t *testing.T) {
	password := []byte("测试口令 with spaces ")
	a, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	b, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if a == b || !strings.HasPrefix(a, hashPrefix) || strings.Contains(a, string(password)) {
		t.Fatal("salt or PHC parameter invariant failed")
	}
	if !VerifyPassword(password, a) || !VerifyPassword(password, b) || VerifyPassword([]byte("wrong"), a) {
		t.Fatal("password verification failed")
	}
	for _, invalid := range []string{"", a + "$extra", strings.Replace(a, "m=65536", "m=999999999", 1), strings.Replace(a, "v=19", "v=18", 1), hashPrefix + "invalid$invalid"} {
		if VerifyPassword(password, invalid) {
			t.Fatal("accepted invalid digest")
		}
	}
	if _, err := HashPassword(nil); err == nil {
		t.Fatal("empty password accepted")
	}
	if VerifyPassword(nil, a) {
		t.Fatal("empty password verified")
	}
}
