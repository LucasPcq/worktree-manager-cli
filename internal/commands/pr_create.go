package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	prwizard "github.com/LucasPcq/wtm/internal/tui/pr"
)

func newPRCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a pull request for the current branch",
		RunE:  runPRCreate,
	}

	cmd.Flags().String(domain.FlagTitle, "", "PR title (skips wizard for this field)")
	cmd.Flags().String(domain.FlagBase, "", "Base branch (skips wizard for this field)")
	cmd.Flags().Bool(domain.FlagDraft, false, "Create as draft PR")

	return cmd
}

func runPRCreate(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	dir, err := projectRoot(cwd)
	if err != nil {
		return err
	}

	result, ok := loadConfig(cmd, dir)
	if !ok {
		return nil
	}

	// Auth check (triggers refresh if needed)
	if _, err := ghservice.ResolveAuth(); err != nil {
		if errors.Is(err, domain.ErrAuthNotConfigured) || errors.Is(err, domain.ErrAuthNeedsReauth) {
			fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
			return nil
		}
		return fmt.Errorf("load auth: %w", err)
	}

	// Resolve current branch from actual cwd (not project root)
	branch, err := infra.CurrentBranch(cwd)
	if err != nil {
		return fmt.Errorf("resolve branch: %w", err)
	}

	// Check for existing PR
	hasPR, prURL := ghservice.HasOpenPR(ghservice.HasOpenPRParams{
		ProjectDir: dir,
		Branch:     branch,
	})
	if hasPR {
		fmt.Fprintf(cmd.ErrOrStderr(), "PR already exists for branch %s\n  %s\n\n", branch, prURL)
		return promptOpenExistingPR(prURL)
	}

	// Check if branch is pushed
	if !infra.BranchExistsOnRemote(infra.BranchExistsOnRemoteParams{
		ProjectDir: dir,
		Branch:     branch,
	}) {
		fmt.Fprintf(cmd.ErrOrStderr(), "Branch %s has not been pushed to origin.\n", branch)
		if err := confirmPush(); err != nil {
			return nil
		}
		stop := startSpinner(cmd.ErrOrStderr(), "Pushing branch...")
		pushErr := infra.PushBranch(infra.PushBranchParams{
			ProjectDir: dir,
			Branch:     branch,
		})
		stop()
		if pushErr != nil {
			return pushErr
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "✓ Pushed %s to origin\n\n", branch)
	}

	// Resolve parameters
	titleFlag, _ := cmd.Flags().GetString(domain.FlagTitle)
	baseFlag, _ := cmd.Flags().GetString(domain.FlagBase)
	draftFlag, _ := cmd.Flags().GetBool(domain.FlagDraft)
	draftChanged := cmd.Flags().Changed(domain.FlagDraft)

	title := titleFlag
	base := baseFlag
	draft := draftFlag

	allFlagsProvided := titleFlag != "" && baseFlag != "" && draftChanged

	if !allFlagsProvided {
		// Resolve defaults for wizard
		defaultTitle := title
		if defaultTitle == "" {
			defaultTitle = ghservice.BranchTitleFromName(branch)
		}

		defaultBase := base
		if defaultBase == "" {
			defaultBase = result.Config.Project.Worktrees.BaseBranch
		}

		autoDraft := result.Config.Project.Github.AutoDraft
		if draftChanged {
			autoDraft = draftFlag
		}

		branches, err := infra.ListLocalBranches(infra.ListBranchesParams{ProjectDir: dir})
		if err != nil {
			return fmt.Errorf("list branches: %w", err)
		}

		wizResult, err := prwizard.RunCreateWizard(prwizard.CreateWizardParams{
			Branches:     branches,
			DefaultBase:  defaultBase,
			AutoDraft:    autoDraft,
			DefaultTitle: defaultTitle,
		})
		if errors.Is(err, domain.ErrUserAborted) {
			return nil
		}
		if err != nil {
			return err
		}

		title = wizResult.Title
		base = wizResult.Base
		draft = wizResult.Draft
	}

	// Detect PR template
	body := ghservice.DetectPRTemplate(dir)

	// Create the PR
	stop := startSpinner(cmd.ErrOrStderr(), "Creating pull request...")
	pr, err := ghservice.CreatePR(ghservice.CreatePRParams{
		ProjectDir: dir,
		Title:      title,
		Body:       body,
		Head:       branch,
		Base:       base,
		Draft:      draft,
	})
	stop()
	if err != nil {
		return err
	}

	state := ""
	if pr.Draft {
		state = " (draft)"
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "✓ PR #%d created%s\n  %s\n", pr.Number, state, pr.URL)

	return nil
}

func promptOpenExistingPR(url string) error {
	var open bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Open in browser?").
				Value(&open),
		),
	)

	if err := form.Run(); err != nil {
		return nil
	}

	if open {
		exec.Command("open", url).Run()
	}

	return nil
}

func confirmPush() error {
	var confirm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Push to origin?").
				Value(&confirm),
		),
	)

	if err := form.Run(); err != nil {
		return domain.ErrUserAborted
	}

	if !confirm {
		return domain.ErrUserAborted
	}

	return nil
}
