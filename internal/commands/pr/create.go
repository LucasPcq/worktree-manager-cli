package pr

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/tui/components"
	prwizard "github.com/LucasPcq/wtm/internal/tui/pr"
)

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdCreate,
		Short: "Create a pull request for the current branch",
		RunE:  runCreate,
	}

	cmd.Flags().String(domain.FlagTitle, "", "PR title (skips wizard for this field)")
	cmd.Flags().String(domain.FlagBase, "", "Base branch (skips wizard for this field)")
	cmd.Flags().Bool(domain.FlagDraft, false, "Create as draft PR")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runCreate(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	dir, err := shared.ProjectRoot(cwd)
	if err != nil {
		return err
	}

	result, ok := shared.LoadConfig(cmd, dir)
	if !ok {
		return nil
	}

	branch, err := infra.CurrentBranch(cwd)
	if err != nil {
		return fmt.Errorf("resolve branch: %w", err)
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	stopCheck := func() {}
	if format != domain.OutputJSON {
		stopCheck = shared.StartSpinner(cmd.ErrOrStderr(), "Checking for existing PR…")
	}
	hasPR, prURL := ghservice.HasOpenPR(ghservice.HasOpenPRParams{
		ProjectDir: dir,
		Branch:     branch,
	})
	stopCheck()
	if hasPR {
		output.Blank(cmd.ErrOrStderr())
		output.Warning(cmd.ErrOrStderr(), fmt.Sprintf("PR already exists for branch %s", branch))
		output.InfoLine(cmd.ErrOrStderr(), "URL", prURL)
		if err := promptOpenExistingPR(prURL); err != nil {
			return err
		}
		output.Blank(cmd.ErrOrStderr())
		return nil
	}

	if !infra.BranchExistsOnRemote(infra.BranchExistsOnRemoteParams{
		ProjectDir: dir,
		Branch:     branch,
	}) {
		output.Warning(cmd.ErrOrStderr(), fmt.Sprintf("Branch %s has not been pushed to origin.", branch))
		if err := confirmPush(); err != nil {
			return nil
		}
		stop := shared.StartSpinner(cmd.ErrOrStderr(), "Pushing branch…")
		pushErr := infra.PushBranch(infra.PushBranchParams{
			ProjectDir: dir,
			Branch:     branch,
		})
		stop()
		if pushErr != nil {
			return pushErr
		}
		output.Success(cmd.ErrOrStderr(), fmt.Sprintf("Pushed %s to origin", branch))
		output.Blank(cmd.ErrOrStderr())
	}

	titleFlag, _ := cmd.Flags().GetString(domain.FlagTitle)
	baseFlag, _ := cmd.Flags().GetString(domain.FlagBase)
	draftFlag, _ := cmd.Flags().GetBool(domain.FlagDraft)
	draftChanged := cmd.Flags().Changed(domain.FlagDraft)

	title := titleFlag
	base := baseFlag
	draft := draftFlag

	allFlagsProvided := titleFlag != "" && baseFlag != "" && draftChanged

	if !allFlagsProvided {
		defaultTitle := title
		if defaultTitle == "" {
			defaultTitle = rules.BranchTitleFromName(branch)
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

	body := ghservice.DetectPRTemplate(dir)

	stop := shared.StartSpinner(cmd.ErrOrStderr(), "Creating pull request…")
	p, err := ghservice.CreatePR(ghservice.CreatePRParams{
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

	if format == domain.OutputJSON {
		return output.WritePRCreateJSON(cmd.OutOrStdout(), p)
	}

	state := ""
	if p.Draft {
		state = " (draft)"
	}
	output.Success(cmd.ErrOrStderr(), fmt.Sprintf("PR #%d created%s", p.Number, state))
	output.InfoLine(cmd.ErrOrStderr(), "URL", p.URL)
	output.Blank(cmd.ErrOrStderr())

	return nil
}

func promptOpenExistingPR(url string) error {
	cm := components.NewConfirm(components.NewConfirmParams{
		Title: "Open in browser?",
	})

	confirmed, err := components.RunStandaloneConfirm(cm)
	if err != nil {
		return nil
	}

	if confirmed {
		exec.Command("open", url).Run()
	}

	return nil
}

func confirmPush() error {
	cm := components.NewConfirm(components.NewConfirmParams{
		Title: "Push to origin?",
	})

	confirmed, err := components.RunStandaloneConfirm(cm)
	if err != nil {
		return domain.ErrUserAborted
	}

	if !confirmed {
		return domain.ErrUserAborted
	}

	return nil
}
