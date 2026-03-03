package main

import (
	_ "embed"
	"fmt"
	"os"
)

//go:embed LICENSE
var projectLicense string

//go:embed THIRD_PARTY_DISCLOSURES.md
var thirdPartyDisclosures string

func printLicenses() {
	_, _ = fmt.Fprintln(os.Stdout, projectLicense)
	_, _ = fmt.Fprintln(os.Stdout, thirdPartyDisclosures)
}
