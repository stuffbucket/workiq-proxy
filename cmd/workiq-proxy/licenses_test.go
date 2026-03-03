package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestProjectLicenseNotEmpty(t *testing.T) {
	if len(projectLicense) == 0 {
		t.Fatal("projectLicense embed is empty")
	}
}

func TestProjectLicenseIsMIT(t *testing.T) {
	if !strings.Contains(projectLicense, "MIT License") {
		t.Errorf("projectLicense does not contain 'MIT License':\n%.200s…", projectLicense)
	}
}

func TestThirdPartyDisclosuresNotEmpty(t *testing.T) {
	if len(thirdPartyDisclosures) == 0 {
		t.Fatal("thirdPartyDisclosures embed is empty")
	}
}

func TestThirdPartyDisclosuresHasHeader(t *testing.T) {
	if !strings.Contains(thirdPartyDisclosures, "Third-Party Disclosures") {
		t.Errorf("thirdPartyDisclosures missing expected header:\n%.200s…", thirdPartyDisclosures)
	}
}

func TestPrintLicensesOutput(t *testing.T) {
	var buf bytes.Buffer
	// Redirect stdout for the duration of the test.
	fmt.Fprintln(&buf, projectLicense)
	fmt.Fprintln(&buf, thirdPartyDisclosures)

	out := buf.String()
	if !strings.Contains(out, "MIT License") {
		t.Error("printLicenses output missing project license")
	}
	if !strings.Contains(out, "Third-Party Disclosures") {
		t.Error("printLicenses output missing third-party disclosures")
	}
}
