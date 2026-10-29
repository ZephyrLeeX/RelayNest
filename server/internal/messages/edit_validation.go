package messages

import (
	"encoding/json"
	"net/url"
	"unicode/utf8"

	"relaynest/server/internal/idempotency"
)

// 受信调用也须校验编码，避免encoding/json将非法UTF-8静默替换为另一正文。
func validateEditTextButtons(in EditTextButtonsInput) (json.RawMessage, error) {
	if in.BotID == "" || !idempotency.ValidKey(in.MessageID) || in.ExpectedRevision < 1 ||
		(in.Format != "plain" && in.Format != "markdown") || !utf8.ValidString(in.Text) {
		return nil, ErrInvalid
	}
	if len(in.Text) > 64*1024 {
		return nil, ErrTooLarge
	}
	if len(in.Buttons) > 10 {
		return nil, ErrInvalid
	}
	ids := make(map[string]bool)
	for _, row := range in.Buttons {
		if len(row) < 1 || len(row) > 5 {
			return nil, ErrInvalid
		}
		for _, b := range row {
			if b.ID == "" || !utf8.ValidString(b.ID) || ids[b.ID] || b.Label == "" ||
				!utf8.ValidString(b.Label) || utf8.RuneCountInString(b.Label) > 128 ||
				(b.URL == nil) == (b.CallbackData == nil) {
				return nil, ErrInvalid
			}
			ids[b.ID] = true
			if b.CallbackData != nil && (!utf8.ValidString(*b.CallbackData) || len(*b.CallbackData) > 4096) {
				return nil, ErrInvalid
			}
			if b.URL != nil {
				if !utf8.ValidString(*b.URL) {
					return nil, ErrInvalid
				}
				parsed, err := url.Parse(*b.URL)
				if err != nil || parsed.Scheme == "" {
					return nil, ErrInvalid
				}
				for _, c := range *b.URL {
					if c <= 0x20 || c == 0x7f {
						return nil, ErrInvalid
					}
				}
			}
		}
	}
	buttons := in.Buttons
	if buttons == nil {
		buttons = [][]Button{}
	}
	return json.Marshal(buttons)
}

func editHasCallback(buttons [][]Button) bool {
	for _, row := range buttons {
		for _, button := range row {
			if button.CallbackData != nil {
				return true
			}
		}
	}
	return false
}
