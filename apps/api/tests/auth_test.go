package tests

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestLoginSessionCookieAndLogout(t *testing.T) {
	e := newEnv(t)
	e.createUser("alice", "requester")
	c := e.anon()

	req, _ := http.NewRequest("POST", e.server.URL+"/api/v1/auth/login",
		strings.NewReader(`{"username":"alice","password":"`+testPassword+`"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login status %d", resp.StatusCode)
	}
	var found bool
	for _, ck := range resp.Cookies() {
		if ck.Name == "rm_session" {
			found = true
			if !ck.HttpOnly {
				t.Error("session cookie must be HttpOnly")
			}
			if ck.SameSite != http.SameSiteStrictMode {
				t.Error("session cookie must be SameSite=Strict")
			}
		}
	}
	if !found {
		t.Fatal("session cookie not set")
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("API responses must not be cacheable, got %q", cc)
	}

	a := e.login("alice")
	a.must(200, "GET", "/api/v1/auth/session", nil)
	a.must(204, "POST", "/api/v1/auth/logout", nil)
	out := a.must(401, "GET", "/api/v1/auth/session", nil)
	if errCode(out) != "unauthorized" {
		t.Fatalf("unexpected %v", out)
	}
}

func TestLoginFailuresAreGenericAndLockAccount(t *testing.T) {
	e := newEnv(t)
	e.createUser("bob", "requester")
	c := e.anon()
	unknown := c.must(401, "POST", "/api/v1/auth/login", map[string]any{"username": "nobody", "password": "whatever-password"})
	for i := 0; i < 5; i++ {
		wrong := c.must(401, "POST", "/api/v1/auth/login", map[string]any{"username": "bob", "password": "wrong-password-x"})
		if errCode(wrong) != errCode(unknown) {
			t.Fatal("failure responses must not reveal whether the account exists")
		}
	}
	// Account is now locked: even the correct password fails, with the same message.
	locked := c.must(401, "POST", "/api/v1/auth/login", map[string]any{"username": "bob", "password": testPassword})
	if errCode(locked) != "login_failed" {
		t.Fatalf("locked account should fail generically: %v", locked)
	}
	// An administrator can unlock it.
	admin := e.login("root", "admin")
	var users map[string]any
	admin.do("GET", "/api/v1/admin/users", nil, &users)
	var bobID string
	for _, u := range users["items"].([]any) {
		um := u.(map[string]any)
		if um["username"] == "bob" {
			bobID = um["id"].(string)
			if um["locked"] != true {
				t.Fatal("bob should be reported as locked")
			}
		}
	}
	admin.must(200, "PATCH", "/api/v1/admin/users/"+bobID, map[string]any{"unlock": true})
	e.login("bob")
}

func TestCSRFAndOriginProtection(t *testing.T) {
	e := newEnv(t)
	c := e.login("carol", "requester")
	token := c.csrf
	c.csrf = ""
	out := c.must(403, "POST", "/api/v1/requests", baseRequest(nil))
	if errCode(out) != "csrf_invalid" {
		t.Fatalf("expected csrf_invalid, got %v", out)
	}
	c.csrf = "forged"
	c.must(403, "POST", "/api/v1/requests", baseRequest(nil))
	c.csrf = token
	var res map[string]any
	if s := c.doWith("POST", "/api/v1/requests", baseRequest(nil), &res, map[string]string{"Origin": "https://evil.example"}); s != 403 {
		t.Fatalf("cross-origin request accepted: %d", s)
	}
	if s := c.doWith("POST", "/api/v1/requests", baseRequest(nil), &res, map[string]string{"Origin": "http://reliefmesh.test"}); s != 201 {
		t.Fatalf("same-origin request rejected: %d %v", s, res)
	}
	// Login CSRF: cross-origin login attempts are rejected too.
	anon := e.anon()
	if s := anon.doWith("POST", "/api/v1/auth/login", map[string]any{"username": "carol", "password": testPassword}, &res,
		map[string]string{"Origin": "https://evil.example"}); s != 403 {
		t.Fatalf("cross-origin login accepted: %d", s)
	}
}

func TestTemporaryPasswordMustBeChanged(t *testing.T) {
	e := newEnv(t)
	admin := e.login("root", "admin")
	created := admin.must(201, "POST", "/api/v1/admin/users", map[string]any{
		"username": "dave", "display_name": "Dave", "roles": []string{"volunteer"}})
	temp := created["temporary_password"].(string)

	c := e.anon()
	res := c.must(200, "POST", "/api/v1/auth/login", map[string]any{"username": "dave", "password": temp})
	c.csrf = res["csrf_token"].(string)
	if res["user"].(map[string]any)["must_change_password"] != true {
		t.Fatal("must_change_password should be set")
	}
	out := c.must(403, "GET", "/api/v1/assignments", nil)
	if errCode(out) != "password_change_required" {
		t.Fatalf("expected password_change_required, got %v", out)
	}
	c.must(422, "POST", "/api/v1/auth/change-password", map[string]any{"current_password": temp, "new_password": "short"})
	c.must(200, "POST", "/api/v1/auth/change-password", map[string]any{"current_password": temp, "new_password": "a much better passphrase"})
	c.must(200, "GET", "/api/v1/assignments", nil)

	// Admin password reset ends sessions and requires a new change.
	var users map[string]any
	admin.do("GET", "/api/v1/admin/users", nil, &users)
	var id string
	for _, u := range users["items"].([]any) {
		if u.(map[string]any)["username"] == "dave" {
			id = u.(map[string]any)["id"].(string)
		}
	}
	reset := admin.must(200, "POST", "/api/v1/admin/users/"+id+"/reset-password", nil)
	if reset["temporary_password"] == "" {
		t.Fatal("no temporary password returned")
	}
	c.must(401, "GET", "/api/v1/assignments", nil)
}

func TestAdminSafeguards(t *testing.T) {
	e := newEnv(t)
	admin := e.login("root", "admin")
	me := admin.userID()
	out := admin.must(409, "PATCH", "/api/v1/admin/users/"+me, map[string]any{"roles": []string{"coordinator"}})
	if errCode(out) != "self_lockout" {
		t.Fatalf("expected self_lockout, got %v", out)
	}
	admin.must(409, "PATCH", "/api/v1/admin/users/"+me, map[string]any{"is_active": false})
	admin.must(422, "POST", "/api/v1/admin/users", map[string]any{"username": "x", "display_name": "X", "roles": []string{"superuser"}})

	// Non-admins cannot reach admin endpoints.
	coord := e.login("coord", "coordinator")
	for _, path := range []string{"/api/v1/admin/users", "/api/v1/admin/audit", "/api/v1/admin/retention", "/api/v1/admin/demo"} {
		coord.must(403, "GET", path, nil)
	}
	coord.must(403, "PUT", "/api/v1/admin/settings", map[string]any{"instance_name": "Hijacked"})
}

func TestSettingsAndDisclaimerConfigurable(t *testing.T) {
	e := newEnv(t)
	anon := e.anon()
	pub := anon.must(200, "GET", "/api/v1/public/notice", nil)
	if !strings.Contains(pub["emergency_notice"].(string), "112") {
		t.Fatalf("default emergency notice must mention 112: %v", pub)
	}
	admin := e.login("root", "admin")
	admin.must(422, "PUT", "/api/v1/admin/settings", map[string]any{"emergency_notice": "short"})
	admin.must(200, "PUT", "/api/v1/admin/settings", map[string]any{
		"emergency_notice": "Emergency notice: in immediate danger call 911. ReliefMesh is not an emergency dispatch service."})
	pub = anon.must(200, "GET", "/api/v1/public/notice", nil)
	if !strings.Contains(pub["emergency_notice"].(string), "911") {
		t.Fatal("notice not updated")
	}
	var n int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE action = 'settings.updated'`).Scan(&n)
	if n != 1 {
		t.Fatalf("settings change must be audited, got %d events", n)
	}
}
