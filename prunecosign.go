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

func getVersions(ctx context.Context, ghOrg, ghUser, packageType, packageName string, client *github.Client) []*github.PackageVersion {
	opt := &github.PackageListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allVersions []*github.PackageVersion

	if ghOrg != "" {
		for {
			versions, resp, err := client.Organizations.PackageGetAllVersions(ctx, ghOrg, packageType, packageName, opt)
			if err != nil {
				slog.Error("Error fetching package versions", "Error", err)
				os.Exit(1)
			}

			allVersions = append(allVersions, versions...)

			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
		return allVersions
	}

	for {
		versions, resp, err := client.Users.PackageGetAllVersions(ctx, ghUser, packageType, packageName, opt)
		if err != nil {
			slog.Error("Error fetching package versions", "Error", err)
			os.Exit(1)
		}

		allVersions = append(allVersions, versions...)

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allVersions
}

func deleteSignature(ctx context.Context, ghOrg, ghUser, packageType, packageName string, client *github.Client, id int64) {
	if ghOrg != "" {
		_, err := client.Organizations.PackageDeleteVersion(ctx, ghOrg, packageType, packageName, id)
		if err != nil {
			slog.Error("Error deleting signature", "Error", err)
			os.Exit(1)
		}
		return
	}
	_, err := client.Users.PackageDeleteVersion(ctx, ghUser, packageType, packageName, id)
	if err != nil {
		slog.Error("Error deleting signature", "Error", err)
		os.Exit(1)
	}
}

func main() {
	ghToken := os.Getenv("GH_TOKEN")
	ghOrg := os.Getenv("GH_ORG")
	ghUser := os.Getenv("GH_USER")
	packageName := os.Getenv("PACKAGE_NAME")
	packageType := os.Getenv("PACKAGE_TYPE")
	dryrun_env := os.Getenv("DRYRUN")

	if ghToken == "" {
		slog.Error("Missing required environment variable: GH_TOKEN")
		os.Exit(1)
	}

	if packageName == "" {
		slog.Error("Missing required environment variable: PACKAGE_NAME")
		os.Exit(1)
	}

	if ghOrg != "" && ghUser != "" {
		slog.Error("Please only provide either a github user or a github org.")
		os.Exit(1)
	}

	if dryrun_env == "" {
		dryrun_env = "false"
	}

	dryrun, err := strconv.ParseBool(dryrun_env)
	if err != nil {
		slog.Error("Error parsing bool", "Error", err)
	}

	if packageType == "" {
		packageType = "container"
	}

	// Setup GitHub client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: ghToken},
	)
	client := github.NewClient(oauth2.NewClient(ctx, ts))

	slog.Info("Fetching Cosign signature tags...")

	versions := getVersions(ctx, ghOrg, ghUser, packageType, packageName, client)

	var remainingDigests []string
	for _, digest := range versions {
		remainingDigests = append(remainingDigests, *digest.Name)
	}

	var signatureVersions []*github.PackageVersion
	for _, v := range versions {
		if len(v.Metadata.Container.Tags) == 0 {
			// Skip this iteration if no tags are available
			continue
		}

		// Check if the tag matches Cosign signature pattern
		if strings.HasPrefix(v.Metadata.Container.Tags[0], "sha256-") && strings.HasSuffix(v.Metadata.Container.Tags[0], ".sig") {
			signatureVersions = append(signatureVersions, v)
			slog.Info("Found Signatures", "Signatures", v.Metadata.Container.Tags[0])
		}
	}

	prunedSigs := ""
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
			slog.Info("Deleting orphaned signature:", "SignatureTag", sigTag)
			prunedSigs += fmt.Sprintf("| %s |\n", sigTag)
			sigDeleted = true

			if !dryrun {
				deleteSignature(ctx, ghOrg, ghUser, packageType, packageName, client, *sig.ID)
			}
		}
	}

	// Append to GitHub summary
	summaryFile := os.Getenv("GITHUB_STEP_SUMMARY")
	f, err := os.OpenFile(summaryFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("Could not open summary file", "Error", err)
	}
	defer f.Close()

	f.WriteString("## Pruned Cosign Signatures\n\n")
	if dryrun {
		f.WriteString(":warning: This is a dry run, no signatures were actually deleted.\n\n")
	}

	if sigDeleted {
		f.WriteString("| Tags |\n|--------------|\n")
		f.WriteString(prunedSigs + "\n")
	} else {
		f.WriteString("No orphaned signatures found.")
	}
}
