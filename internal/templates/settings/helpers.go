package settingsview

import "fmt"

func smtpPortValue(port int) string {
	if port == 0 {
		return "587"
	}
	return fmt.Sprintf("%d", port)
}

// smtpPasswordPlaceholder shows a masked placeholder if a password is
// already stored, so the form doesn't display it in plaintext but also
// doesn't force re-entering it on every unrelated field change (the
// handler leaves the stored password untouched if this field is submitted
// empty).
func smtpPasswordPlaceholder(existing string) string {
	if existing != "" {
		return "•••••••• (unchanged)"
	}
	return ""
}
