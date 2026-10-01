package locations

import "github.com/6-slx-6/reliefmesh/apps/api/internal/validation"

// Contact visibility values.
const (
	ContactVisibilityNone               = "none"
	ContactVisibilityCoordinatorsOnly   = "coordinators_only"
	ContactVisibilityAssignedResponders = "assigned_responders"
)

// ContactVisibilities lists valid visibilities.
var ContactVisibilities = []string{ContactVisibilityNone, ContactVisibilityCoordinatorsOnly, ContactVisibilityAssignedResponders}

// ContactMethods lists valid contact methods.
var ContactMethods = []string{"none", "phone", "messenger", "email", "in_person", "via_shelter_desk", "other"}

// ContactInput is the client-supplied contact information. Details are
// protected and stored encrypted.
type ContactInput struct {
	Method     string `json:"method"`
	Visibility string `json:"visibility"`
	Details    string `json:"details"`
	// KeepDetails keeps previously stored details on update without the
	// client having to reveal and resend them.
	KeepDetails bool `json:"keep_details,omitempty"`
}

// NormalizedContact is validated contact information.
type NormalizedContact struct {
	Method     string
	Visibility string
	Details    string
	Keep       bool
}

// NormalizeContact validates contact input and applies data minimization:
// when the person does not want to be contacted, no details are kept.
func NormalizeContact(in ContactInput, errs validation.Errors) NormalizedContact {
	c := NormalizedContact{
		Method:     in.Method,
		Visibility: in.Visibility,
		Details:    validation.CleanText(in.Details),
		Keep:       in.KeepDetails,
	}
	if c.Method == "" {
		c.Method = "none"
	}
	if c.Visibility == "" {
		c.Visibility = ContactVisibilityCoordinatorsOnly
	}
	errs.OneOf("contact.method", c.Method, ContactMethods)
	errs.OneOf("contact.visibility", c.Visibility, ContactVisibilities)
	errs.Text("contact.details", c.Details, 0, 300, true)
	if c.Method == "none" || c.Visibility == ContactVisibilityNone {
		c.Details, c.Keep = "", false
		if c.Visibility == ContactVisibilityNone {
			c.Method = "none"
		}
	}
	return c
}
