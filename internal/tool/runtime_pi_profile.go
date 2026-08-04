package tool

import "errors"

// Profile is a Host-only read of a pre-registered profile. It exists for
// preflight identity verification and never crosses the provider tool API.
func (h *PiRuntimeHost) Profile(ref string) (PiRuntimeProfile, error) {
	if h == nil || ref == "" {
		return PiRuntimeProfile{}, errors.New("Pi runtime profile is absent")
	}
	profile := h.profiles[ref]
	if profile.Ref == "" {
		return PiRuntimeProfile{}, errors.New("Pi runtime profile is absent")
	}
	return profile, nil
}
