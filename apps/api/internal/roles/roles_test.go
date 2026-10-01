package roles

import "testing"

func TestAdminHasNoOperationalAccess(t *testing.T) {
	p := Principal{Roles: []Role{Admin}}
	for _, c := range []Capability{CapRequestReadAll, CapRequestManage, CapOfferReadAll, CapAssignmentManage,
		CapRequestCreate, CapDashboardQueues, CapExportReports} {
		if p.Can(c) {
			t.Errorf("admin must not have %s", c)
		}
	}
	if !p.Can(CapUserManage) || !p.Can(CapSettingsManage) {
		t.Fatal("admin must manage users and settings")
	}
}

func TestVolunteerLeastPrivilege(t *testing.T) {
	p := Principal{Roles: []Role{Volunteer}}
	for _, c := range []Capability{CapRequestReadAll, CapOfferReadAll, CapAssignmentManage, CapVolunteerList} {
		if p.Can(c) {
			t.Errorf("volunteer must not have %s", c)
		}
	}
	if !p.Can(CapAssignmentWorkOwn) {
		t.Fatal("volunteer should work on own assignments")
	}
}

func TestOrganizationManagerExtendsCoordinator(t *testing.T) {
	coord := Principal{Roles: []Role{Coordinator}}
	mgr := Principal{Roles: []Role{OrganizationManager}}
	for _, c := range CapabilitiesFor([]Role{Coordinator}) {
		if !mgr.Can(c) {
			t.Errorf("organization manager lacks coordinator capability %s", c)
		}
	}
	if coord.Can(CapExportReports) || coord.Can(CapVolunteerAvailability) || coord.Can(CapUserManage) {
		t.Fatal("coordinator must not export, manage availability or users")
	}
	if !mgr.Can(CapExportReports) || mgr.Can(CapSettingsManage) {
		t.Fatal("organization manager exports but cannot change security settings")
	}
}

func TestParse(t *testing.T) {
	rs, err := Parse([]string{"admin", "volunteer", "admin"})
	if err != nil || len(rs) != 2 || rs[0] != Volunteer {
		t.Fatalf("unexpected %v %v", rs, err)
	}
	if _, err := Parse([]string{"root"}); err == nil {
		t.Fatal("unknown role accepted")
	}
	if _, err := Parse(nil); err == nil {
		t.Fatal("empty roles accepted")
	}
}
