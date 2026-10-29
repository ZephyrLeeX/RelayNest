package messages

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"relaynest/server/internal/bots"
)

func TestEditTextButtonsValidation(t *testing.T) {
	pointer := func(s string) *string { return &s }
	cases := map[string]func(*EditTextButtonsInput){
		"format":         func(in *EditTextButtonsInput) { in.Format = "html" },
		"id":             func(in *EditTextButtonsInput) { in.MessageID = "invalid" },
		"revision":       func(in *EditTextButtonsInput) { in.ExpectedRevision = 0 },
		"text-utf8":      func(in *EditTextButtonsInput) { in.Text = string([]byte{255}) },
		"text-limit":     func(in *EditTextButtonsInput) { in.Text = strings.Repeat("界", 21846) },
		"row-empty":      func(in *EditTextButtonsInput) { in.Buttons = [][]Button{{}} },
		"rows-limit":     func(in *EditTextButtonsInput) { in.Buttons = make([][]Button, 11) },
		"row-limit":      func(in *EditTextButtonsInput) { in.Buttons = [][]Button{make([]Button, 6)} },
		"id-empty":       func(in *EditTextButtonsInput) { in.Buttons[0][0].ID = "" },
		"id-utf8":        func(in *EditTextButtonsInput) { in.Buttons[0][0].ID = string([]byte{255}) },
		"duplicate":      func(in *EditTextButtonsInput) { in.Buttons = append(in.Buttons, in.Buttons[0]) },
		"label-empty":    func(in *EditTextButtonsInput) { in.Buttons[0][0].Label = "" },
		"label-utf8":     func(in *EditTextButtonsInput) { in.Buttons[0][0].Label = string([]byte{255}) },
		"label-limit":    func(in *EditTextButtonsInput) { in.Buttons[0][0].Label = strings.Repeat("界", 129) },
		"missing-action": func(in *EditTextButtonsInput) { in.Buttons[0][0].URL = nil },
		"two-actions":    func(in *EditTextButtonsInput) { in.Buttons[0][0].CallbackData = pointer("") },
		"url-relative":   func(in *EditTextButtonsInput) { in.Buttons[0][0].URL = pointer("/relative") },
		"url-space":      func(in *EditTextButtonsInput) { in.Buttons[0][0].URL = pointer("https://example.test/a b") },
		"url-control":    func(in *EditTextButtonsInput) { in.Buttons[0][0].URL = pointer("scheme:a\x7f") },
		"url-escape":     func(in *EditTextButtonsInput) { in.Buttons[0][0].URL = pointer("https://example.test/%XX") },
		"url-utf8":       func(in *EditTextButtonsInput) { in.Buttons[0][0].URL = pointer("scheme:" + string([]byte{255})) },
		"callback-limit": func(in *EditTextButtonsInput) {
			in.Buttons[0][0].URL = nil
			in.Buttons[0][0].CallbackData = pointer(strings.Repeat("界", 1366))
		},
		"callback-utf8": func(in *EditTextButtonsInput) {
			in.Buttons[0][0].URL = nil
			in.Buttons[0][0].CallbackData = pointer(string([]byte{255}))
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := setupMessages(t)
			_, in := editSeed(t, f)
			in.Buttons = [][]Button{{{ID: "id", Label: "label", URL: pointer("https://example.test/path")}}}
			mutate(&in)
			before := editSnapshot(t, f.other)
			out, err := editOwned(messageContext, f.s, f.r, f.identity, in)
			if err == nil || !reflect.DeepEqual(out, Edited{}) || !reflect.DeepEqual(before, editSnapshot(t, f.other)) {
				t.Fatal("invalid input wrote or leaked", err)
			}
		})
	}
	f := setupMessages(t)
	_, in := editSeed(t, f)
	in.Text = strings.Repeat("界", 21845) + "x"
	callback := strings.Repeat("界", 1365) + "x"
	for row := 0; row < 10; row++ {
		buttons := make([]Button, 5)
		for col := range buttons {
			buttons[col] = Button{ID: fmt.Sprintf("%d-%d", row, col), Label: strings.Repeat("界", 128), CallbackData: &callback}
		}
		in.Buttons = append(in.Buttons, buttons)
	}
	out, err := editOwned(messageContext, f.s, f.r, f.identity, in)
	if err != nil || out.Message.Revision != 2 || out.Message.Text != in.Text {
		t.Fatal("inclusive frozen boundaries rejected", err)
	}
}

func TestEditTextButtonsCallbackReceiveBoundary(t *testing.T) {
	for _, mode := range []string{"url-only", "callback", "snapshot-no-gain", "final-revoked"} {
		t.Run(mode, func(t *testing.T) {
			f := setupMessages(t)
			_, in := editSeed(t, f)
			callback, link := "  unchanged\n", "https://example.test/"
			in.Buttons = [][]Button{{{ID: "button", Label: "label", CallbackData: &callback}}}
			identity := f.identity
			if mode == "url-only" {
				in.Buttons[0][0].CallbackData = nil
				in.Buttons[0][0].URL = &link
			}
			if mode == "snapshot-no-gain" {
				execMessages(t, f.s, `UPDATE bots SET can_receive=0 WHERE id=?`, f.a.Bot.ID)
				var err error
				identity, err = f.b.Authenticate(messageContext, f.a.Bot.ID, bots.APIToken, f.a.APIToken.Token)
				if err != nil {
					t.Fatal(err)
				}
				execMessages(t, f.s, `UPDATE bots SET can_receive=1 WHERE id=?`, f.a.Bot.ID)
			} else if mode == "final-revoked" {
				execMessages(t, f.s, `CREATE TRIGGER edit_callback_revoke AFTER INSERT ON changes BEGIN UPDATE bots SET can_receive=0 WHERE id=NEW.bot_id;END`)
			} else {
				execMessages(t, f.s, `UPDATE bots SET can_receive=0 WHERE id=?`, f.a.Bot.ID)
			}
			before := editSnapshot(t, f.other)
			out, err := editOwned(messageContext, f.s, f.r, identity, in)
			if mode == "url-only" {
				if err != nil || out.Message.Revision != 2 {
					t.Fatal("URL-only wrongly required receive", err)
				}
			} else if !errors.Is(err, bots.ErrForbidden) || !reflect.DeepEqual(out, Edited{}) || !reflect.DeepEqual(before, editSnapshot(t, f.other)) {
				t.Fatal("callback bypassed receive", err)
			}
		})
	}
}

func TestEditTextButtonsQueuedRevocation(t *testing.T) {
	for _, mutation := range []string{`UPDATE bots SET can_send=0`, `UPDATE bots SET enabled=0`, `UPDATE bot_credentials SET generation=generation+1`} {
		t.Run(mutation, func(t *testing.T) {
			f := setupMessages(t)
			_, in := editSeed(t, f)
			held, release := make(chan struct{}), make(chan struct{})
			owner := make(chan error, 1)
			go func() {
				owner <- f.other.Write(messageContext, func(tx *sql.Tx) error { _, err := tx.Exec(mutation); close(held); <-release; return err })
			}()
			<-held
			result := make(chan error, 1)
			go func() {
				out, err := editOwned(messageContext, f.s, f.r, f.identity, in)
				if !reflect.DeepEqual(out, Edited{}) {
					result <- fmt.Errorf("leaked candidate")
					return
				}
				result <- err
			}()
			close(release)
			if err := <-owner; err != nil {
				t.Fatal(err)
			}
			before := editSnapshot(t, f.other)
			if err := <-result; err == nil {
				t.Fatal("queued revoked edit succeeded")
			}
			if !reflect.DeepEqual(before, editSnapshot(t, f.other)) {
				t.Fatal("queued rejection changed state")
			}
		})
	}
}
