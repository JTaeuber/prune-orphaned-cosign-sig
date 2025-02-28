package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"
)

func main() {
	ghToken := os.Getenv("GH_TOKEN")
	ghOrg := os.Getenv("GH_ORG")
	packageName := os.Getenv("PACKAGE_NAME")
	packageType := os.Getenv("PACKAGE_TYPE")
	dryrun_env := os.Getenv("DRYRUN")

	if dryrun_env == "" {
		dryrun_env = "false"
	}

	dryrun, err := strconv.ParseBool(dryrun_env)
	if err != nil {
		slog.Error("Error parsing bool", "Error", err)
	}

	if ghToken == "" {
		ghToken = os.Getenv("GITHUB_TOKEN")
	}

	if ghOrg == "" {
		ghOrg = os.Getenv("GITHUB_REPOSITORY_OWNER")
	}

	if packageType == "" {
		packageType = "container"
	}

	if packageName == "" {
		slog.Error("Missing required environment variable: IMAGE_NAME")
		os.Exit(1)
	}

	// Setup GitHub client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: ghToken},
	)
	client := github.NewClient(oauth2.NewClient(ctx, ts))

	// Fetch image digests
	slog.Info("Fetching image digests...")

	// Get image versions (digests)
	versions, _, err := client.Organizations.PackageGetAllVersions(ctx, ghOrg, packageType, packageName, &github.PackageListOptions{})
	if err != nil {
		slog.Error("Error fetching package versions", "Error", err)
		os.Exit(1)
	}

	var remainingDigests []string
	for _, version := range versions {
		remainingDigests = append(remainingDigests, *version.Name)
	}

	// Fetch Cosign signature tags
	slog.Info("Fetching Cosign signature tags...")

	signatures, _, err := client.Organizations.PackageGetAllVersions(ctx, ghOrg, packageType, packageName, &github.PackageListOptions{})
	if err != nil {
		slog.Error("Error fetching Cosign signatures:", "Error", err)
		os.Exit(1)
	}

	var signatureVersions []*github.PackageVersion
	for _, signature := range signatures {
		// Check if the tag matches Cosign signature pattern
		if matched := strings.HasPrefix(signature.Metadata.Container.Tags[0], "sha256-") && strings.HasSuffix(signature.Metadata.Container.Tags[0], ".sig"); matched {
			signatureVersions = append(signatureVersions, signature)
		}
	}

	// Prepare to delete orphaned Cosign signatures
	prunedSigs := "### Pruned Cosign Signatures\n\n"
	sigDeleted := false

	for _, sig := range signatureVersions {
		sigTag := sig.Metadata.Container.Tags[0]
		sigDigest := strings.TrimPrefix(sigTag, "sha256-")
		sigDigest = strings.TrimSuffix(sigDigest, ".sig")
		sigDigest = fmt.Sprintf("sha256:%s", sigDigest)

		// Check if the digest is missing in the remaining digests
		found := false
		for _, digest := range remainingDigests {
			if sigDigest == digest {
				found = true
				break
			}
		}

		if !found {
			// Orphaned signature found, delete it
			slog.Info("Deleting orphaned signature:", "SignatureTag", sigTag)
			prunedSigs += fmt.Sprintf("- %s\n", sigTag)
			sigDeleted = true

			if !dryrun {
				_, err := client.Organizations.PackageDeleteVersion(ctx, ghOrg, packageType, packageName, *sig.ID)
				if err != nil {
					slog.Error("Error deleting signature", "Error", err)
					os.Exit(1)
				}
			}
		}
	}

	// Append to GitHub summary only if signatures were deleted
	if sigDeleted {
		if dryrun {
			fmt.Println("This is a dry run, no signatures were actually deleted.")
		}

		fmt.Println("Deleted orphaned signatures")
		fmt.Println(prunedSigs)
	} else {
		fmt.Println("No orphaned signatures found.")
	}
}
