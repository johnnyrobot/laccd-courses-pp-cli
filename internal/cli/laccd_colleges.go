// Copyright 2026 johnnyrobot and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored LACCD college mapping (standalone file so regen preserves it).

package cli

import "strings"

// laccdCollege maps a friendly LACCD college code to its eLumen tenant host.
// The 9 colleges are fixed institutional data; several host names are not
// guessable from the code (LAMC -> lamission, LAPC -> pierce), which is exactly
// why the friendly --tenant mapping is worth shipping.
type laccdCollege struct {
	Code string
	Host string
	Name string
}

var laccdColleges = []laccdCollege{
	{"ELAC", "elac.elumenapp.com", "East Los Angeles College"},
	{"LACC", "lacc.elumenapp.com", "Los Angeles City College"},
	{"LAHC", "lahc.elumenapp.com", "Los Angeles Harbor College"},
	{"LAMC", "lamission.elumenapp.com", "Los Angeles Mission College"},
	{"LAPC", "pierce.elumenapp.com", "Los Angeles Pierce College"},
	{"LASC", "lasc.elumenapp.com", "Los Angeles Southwest College"},
	{"LATTC", "lattc.elumenapp.com", "Los Angeles Trade-Technical College"},
	{"LAVC", "lavc.elumenapp.com", "Los Angeles Valley College"},
	{"WLAC", "wlac.elumenapp.com", "West Los Angeles College"},
}

// laccdCollegeHosts returns all 9 eLumen tenant hosts in stable order.
func laccdCollegeHosts() []string {
	hosts := make([]string, 0, len(laccdColleges))
	for _, c := range laccdColleges {
		hosts = append(hosts, c.Host)
	}
	return hosts
}

// resolveTenant maps a user-supplied college identifier to its eLumen tenant
// host. It accepts the friendly code (ELAC, elac, Pierce, LAPC), the bare
// subdomain (pierce), or the full host (pierce.elumenapp.com). Unknown values
// pass through unchanged so power users can still target any host directly.
func resolveTenant(input string) string {
	in := strings.TrimSpace(input)
	if in == "" {
		return ""
	}
	lower := strings.ToLower(in)
	for _, c := range laccdColleges {
		if strings.EqualFold(in, c.Code) || lower == c.Host || lower == strings.TrimSuffix(c.Host, ".elumenapp.com") {
			return c.Host
		}
	}
	// "Pierce" / "Mission" style name fragments.
	for _, c := range laccdColleges {
		host := strings.TrimSuffix(c.Host, ".elumenapp.com")
		if lower == host || strings.Contains(strings.ToLower(c.Name), lower) && len(lower) >= 4 {
			return c.Host
		}
	}
	return in
}

// collegeCodeForHost returns the friendly code for an eLumen host, or the host
// itself when it is not a known LACCD college.
func collegeCodeForHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	for _, c := range laccdColleges {
		if h == c.Host {
			return c.Code
		}
	}
	return host
}
