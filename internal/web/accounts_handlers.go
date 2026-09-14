package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wkulhanek/kulhanek-points-tracker/internal/accounts"
	"github.com/wkulhanek/kulhanek-points-tracker/internal/templates/accounts"
)

func handleAccountsList(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prefs, _ := d.Preferences.Get()
		accts, err := d.Accounts.List()
		if err != nil {
			http.Error(w, "failed to load accounts", http.StatusInternalServerError)
			return
		}
		kind, msg := flashFromQuery(r)
		render(w, r, accountsview.List(prefs.AppDisplayName, accts, kind, msg))
	}
}

func handleAccountNewForm(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		render(w, r, accountsview.Form(accounts.Account{}, true, ""))
	}
}

func handleAccountCreate(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := parseAccountInput(r)
		if err != nil {
			render(w, r, accountsview.Form(accounts.Account{}, true, err.Error()))
			return
		}

		id, err := d.Accounts.Create(in)
		if err != nil {
			render(w, r, accountsview.Form(accounts.Account{}, true, "Failed to save account."))
			return
		}

		a, err := d.Accounts.Get(id)
		if err != nil {
			http.Error(w, "failed to load saved account", http.StatusInternalServerError)
			return
		}

		render(w, r, accountsview.Row(a))
		// A successful create always means the list is non-empty now, so
		// OOB-swap away the "No accounts yet" message without an extra query.
		render(w, r, accountsview.EmptyStateOOB(false))
		// Clear the add-account form slot out-of-band now that it's saved.
		fmt.Fprint(w, `<div id="account-form-slot" hx-swap-oob="true"></div>`)
	}
}

func handleAccountEditForm(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		a, err := d.Accounts.Get(id)
		if errors.Is(err, accounts.ErrNotFound) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			http.Error(w, "failed to load account", http.StatusInternalServerError)
			return
		}
		render(w, r, accountsview.Form(a, false, ""))
	}
}

// handleAccountView renders a single row back in its normal (non-editing)
// state — used by the edit form's Cancel button to revert without a full
// page reload.
func handleAccountView(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		a, err := d.Accounts.Get(id)
		if errors.Is(err, accounts.ErrNotFound) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			http.Error(w, "failed to load account", http.StatusInternalServerError)
			return
		}

		if r.Header.Get("HX-Request") == "" {
			http.Redirect(w, r, "/accounts", http.StatusSeeOther)
			return
		}
		render(w, r, accountsview.Row(a))
	}
}

func handleAccountUpdate(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		in, err := parseAccountInput(r)
		if err != nil {
			a, _ := d.Accounts.Get(id)
			a.ID = id
			render(w, r, accountsview.Form(a, false, err.Error()))
			return
		}

		if err := d.Accounts.Update(id, in); err != nil {
			a, _ := d.Accounts.Get(id)
			a.ID = id
			render(w, r, accountsview.Form(a, false, "Failed to save account."))
			return
		}

		a, err := d.Accounts.Get(id)
		if err != nil {
			http.Error(w, "failed to load saved account", http.StatusInternalServerError)
			return
		}
		render(w, r, accountsview.Row(a))
	}
}

func handleAccountDelete(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if err := d.Accounts.Delete(id); err != nil {
			http.Error(w, "failed to delete account", http.StatusInternalServerError)
			return
		}
		accts, err := d.Accounts.List()
		if err != nil {
			http.Error(w, "failed to load accounts", http.StatusInternalServerError)
			return
		}
		// Empty body + hx-swap="outerHTML" on the row removes it from the
		// table; separately OOB-swap the "No accounts yet" message back in
		// if that was the last account.
		render(w, r, accountsview.EmptyStateOOB(len(accts) == 0))
	}
}

func parseAccountInput(r *http.Request) (accounts.Input, error) {
	if err := r.ParseForm(); err != nil {
		return accounts.Input{}, errors.New("invalid form submission.")
	}

	name := strings.TrimSpace(r.FormValue("name"))
	provider := strings.TrimSpace(r.FormValue("provider"))
	if name == "" || provider == "" {
		return accounts.Input{}, errors.New("name and provider are required.")
	}

	points, err := strconv.ParseInt(r.FormValue("points_balance"), 10, 64)
	if err != nil || points < 0 {
		return accounts.Input{}, errors.New("points balance must be a non-negative number.")
	}

	owner := accounts.Owner(r.FormValue("owner"))
	if !validOwner(owner) {
		return accounts.Input{}, errors.New("owner must be Wolfgang, Barbara, or Joint.")
	}

	doesNotExpire := r.FormValue("does_not_expire") != ""

	var expiration time.Time
	if !doesNotExpire {
		expiration, err = time.Parse("2006-01-02", r.FormValue("expiration_date"))
		if err != nil {
			return accounts.Input{}, errors.New("expiration date is required (or check \"Does not expire\").")
		}
	}

	return accounts.Input{
		Name:           name,
		Provider:       provider,
		AccountNumber:  strings.TrimSpace(r.FormValue("account_number")),
		PointsBalance:  points,
		ExpirationDate: expiration,
		DoesNotExpire:  doesNotExpire,
		Owner:          owner,
		Notes:          strings.TrimSpace(r.FormValue("notes")),
	}, nil
}

func validOwner(o accounts.Owner) bool {
	for _, valid := range accounts.Owners {
		if o == valid {
			return true
		}
	}
	return false
}
