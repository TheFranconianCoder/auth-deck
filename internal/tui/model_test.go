package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/TheFranconianCoder/auth-deck/internal/state"
)

func TestProviderKey(t *testing.T) {
	cases := []struct {
		index int
		want  string
	}{
		{-1, ""},
		{0, "1"},
		{8, "9"},
		{9, "a"},
		{10, "b"},
		{34, "z"},
		{35, ""},
		{40, ""},
	}
	for _, c := range cases {
		if got := providerKey(c.index); got != c.want {
			t.Errorf("providerKey(%d) = %q, want %q", c.index, got, c.want)
		}
	}
}

func TestProviderIndexRoundTrip(t *testing.T) {
	for i := 0; i < 35; i++ {
		key := providerKey(i)
		got, ok := providerIndex(key)
		if !ok || got != i {
			t.Errorf("providerIndex(%q) = %d, %v; want %d, true", key, got, ok, i)
		}
	}
	for _, invalid := range []string{"0", "A", "ab", "", " "} {
		if _, ok := providerIndex(invalid); ok {
			t.Errorf("providerIndex(%q) unexpectedly ok", invalid)
		}
	}
}

func TestEnsureProviderVisible(t *testing.T) {
	m := Model{providers: make([]providerStatus, 40)}

	m.providerCursor = 30
	m.ensureProviderVisible()
	if want := 30 - maxProviderRows + 1; m.providerOffset != want {
		t.Fatalf("offset = %d, want %d", m.providerOffset, want)
	}

	m.providerCursor = 2
	m.ensureProviderVisible()
	if m.providerOffset != 2 {
		t.Fatalf("offset = %d, want 2", m.providerOffset)
	}

	m.providerCursor = 0
	m.ensureProviderVisible()
	if m.providerOffset != 0 {
		t.Fatalf("offset = %d, want 0", m.providerOffset)
	}
}

func namedStatuses(n int) []providerStatus {
	statuses := make([]providerStatus, n)
	for i := range statuses {
		statuses[i].name = fmt.Sprintf("p%d", i)
	}
	return statuses
}

func TestLetterShortcutSelectsProvider(t *testing.T) {
	req := &state.PendingRequest{ProviderCh: make(chan state.Decision, 1)}
	m := Model{
		providers: namedStatuses(12),
		pending:   []*state.PendingRequest{req},
		focus:     focusProviders,
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})

	select {
	case d := <-req.ProviderCh:
		if d.Provider != "p10" {
			t.Fatalf("provider = %q, want p10", d.Provider)
		}
	default:
		t.Fatal("no decision sent")
	}
}

func TestQSelectsProviderInsteadOfQuitting(t *testing.T) {
	req := &state.PendingRequest{ProviderCh: make(chan state.Decision, 1)}
	m := Model{
		providers: namedStatuses(35),
		pending:   []*state.PendingRequest{req},
		focus:     focusProviders,
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd != nil {
		t.Fatal("q must not quit")
	}

	select {
	case d := <-req.ProviderCh:
		if d.Provider != "p25" {
			t.Fatalf("provider = %q, want p25", d.Provider)
		}
	default:
		t.Fatal("no decision sent")
	}
}

func TestEscRejectsPendingRequest(t *testing.T) {
	req := &state.PendingRequest{Method: "GET", Path: "/x", ProviderCh: make(chan state.Decision, 1)}
	m := Model{pending: []*state.PendingRequest{req}}

	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	select {
	case d := <-req.ProviderCh:
		if !d.Rejected {
			t.Fatal("expected rejection")
		}
	default:
		t.Fatal("no decision sent")
	}
}

func TestEscClearsNoticeWithoutPending(t *testing.T) {
	m := Model{notice: "hello"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if got := updated.(Model).notice; got != "" {
		t.Fatalf("notice = %q, want empty", got)
	}
}

func TestTabTogglesFocus(t *testing.T) {
	m := Model{
		pending: []*state.PendingRequest{{ProviderCh: make(chan state.Decision, 1)}},
		focus:   focusProviders,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := updated.(Model).focus; got != focusPending {
		t.Fatalf("focus = %v, want focusPending", got)
	}

	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := updated.(Model).focus; got != focusProviders {
		t.Fatalf("focus = %v, want focusProviders", got)
	}
}

func TestTabWithoutPendingStaysOnProviders(t *testing.T) {
	m := Model{focus: focusProviders}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})

	if got := updated.(Model).focus; got != focusProviders {
		t.Fatalf("focus = %v, want focusProviders", got)
	}
}

func TestFocusColors(t *testing.T) {
	if got := providerBorderColor(focusProviders); got != lipgloss.Color("62") {
		t.Errorf("providerBorderColor(focusProviders) = %q", got)
	}
	if got := providerBorderColor(focusPending); got != lipgloss.Color("240") {
		t.Errorf("providerBorderColor(focusPending) = %q", got)
	}

	if got := pendingBorderColor(focusPending); got != lipgloss.Color("228") {
		t.Errorf("pendingBorderColor(focusPending) = %q", got)
	}
	if got := pendingBorderColor(focusProviders); got != lipgloss.Color("240") {
		t.Errorf("pendingBorderColor(focusProviders) = %q", got)
	}

	if got := pendingCursorColor(focusPending); got != lipgloss.Color("205") {
		t.Errorf("pendingCursorColor(focusPending) = %q", got)
	}
	if got := pendingCursorColor(focusProviders); got != lipgloss.Color("240") {
		t.Errorf("pendingCursorColor(focusProviders) = %q", got)
	}
}

func TestRequestDoneResetsFocus(t *testing.T) {
	req := &state.PendingRequest{ID: "req-1", ProviderCh: make(chan state.Decision, 1)}
	m := Model{
		pending: []*state.PendingRequest{req},
		focus:   focusPending,
	}

	updated, _ := m.Update(state.RequestDone{ID: "req-1"})
	got := updated.(Model)

	if got.focus != focusProviders {
		t.Fatalf("focus = %v, want focusProviders", got.focus)
	}
	if len(got.pending) != 0 {
		t.Fatalf("pending = %d, want 0", len(got.pending))
	}
}
