package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/WenqiOfficial/qobuz-dl-go/internal/api"
	"github.com/WenqiOfficial/qobuz-dl-go/internal/config"
)

type authDiagnostic struct {
	Step       string `json:"step"`
	Status     string `json:"status"`
	Detail     string `json:"detail,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

func newAuthCommand() *cobra.Command {
	var jsonOutput bool
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Inspect Qobuz authentication",
	}
	doctorCmd := &cobra.Command{
		Use:           "doctor",
		Short:         "Diagnose app credentials, token, and login connectivity",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthDoctor(cmd.OutOrStdout(), jsonOutput)
		},
	}
	doctorCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output diagnostics as JSON")
	authCmd.AddCommand(doctorCmd)
	return authCmd
}

func runAuthDoctor(output io.Writer, jsonOutput bool) error {
	reports := make([]authDiagnostic, 0, 6)
	add := func(step, status, detail, suggestion string) {
		reports = append(reports, authDiagnostic{Step: step, Status: status, Detail: detail, Suggestion: suggestion})
	}
	manager := newAuthManager(authOptions(true, true))
	if err := manager.LoadAccount(); err != nil {
		add("account", "error", fmt.Sprintf("Cannot read %s: %v", config.GetAccountPath(), err), "Repair or move the invalid account file, then retry.")
		return writeAuthDiagnostics(output, jsonOutput, reports, err)
	}
	add("account", "ok", fmt.Sprintf("Loaded %s", config.GetAccountPath()), "")

	client, acc, err := manager.PrepareDiagnostics()
	if err != nil {
		status, suggestion := diagnoseAuthError(err)
		add("authentication", status, err.Error(), suggestion)
		return writeAuthDiagnostics(output, jsonOutput, reports, err)
	}

	add("app_id", "ok", fmt.Sprintf("Using %s (%s)", client.AppID, credentialSource(acc.AppID)), "")
	add("secret", "ok", "Qobuz accepted the request signature.", "")

	userToken := strings.TrimSpace(flagToken)
	if userToken == "" {
		userToken = acc.UserToken
	}
	if userToken == "" {
		add("token", "missing", "No authenticated user token is available.", "Use --email/--password or --token to log in.")
		return writeAuthDiagnostics(output, jsonOutput, reports, nil)
	}

	response, tokenErr := manager.ValidateUserToken(userToken)
	if tokenErr != nil {
		status, suggestion := diagnoseAuthError(tokenErr)
		add("token", status, tokenErr.Error(), suggestion)
		return writeAuthDiagnostics(output, jsonOutput, reports, tokenErr)
	}
	if response == nil || strings.TrimSpace(response.UserAuthToken) == "" {
		tokenErr = fmt.Errorf("token validation returned an empty response: %w", api.ErrInvalidResponse)
		add("token", "invalid_response", tokenErr.Error(), diagnoseSuggestion(api.ErrInvalidResponse))
		return writeAuthDiagnostics(output, jsonOutput, reports, tokenErr)
	}
	add("token", "ok", fmt.Sprintf("Token accepted for App ID %s.", client.AppID), "")
	return writeAuthDiagnostics(output, jsonOutput, reports, nil)
}

func diagnoseAuthError(err error) (string, string) {
	switch {
	case errors.Is(err, api.ErrInvalidSecret):
		return "invalid_secret", "Refresh the app credentials; do not treat this as a password failure."
	case errors.Is(err, api.ErrAppIDRejected):
		return "app_id_mismatch", "Use the App ID that issued the token with --app-id, or log in again."
	case errors.Is(err, api.ErrTokenInvalid):
		return "token_invalid", "Log in again with --email/--password or provide a fresh --token."
	case errors.Is(err, api.ErrBadCredentials):
		return "bad_credentials", "Check the email and password; the app secret is not refreshed for this error."
	case errors.Is(err, api.ErrForbidden):
		return "forbidden", "Check the subscription, account entitlement, and region."
	case errors.Is(err, api.ErrNotFound):
		return "not_found", "Check the resource ID or validation track; credentials are not rotated."
	case errors.Is(err, api.ErrRateLimited):
		return "rate_limited", "Wait before retrying; do not repeatedly submit the password."
	case errors.Is(err, api.ErrInvalidResponse):
		return "invalid_response", "Check the built-in proxy or use --noproxy to connect directly."
	case errors.Is(err, api.ErrTransient):
		return "temporary_failure", "Check Qobuz availability and retry."
	default:
		return "connection_error", "Check network connectivity and --proxy settings."
	}
}

func diagnoseSuggestion(err error) string {
	_, suggestion := diagnoseAuthError(err)
	return suggestion
}

func credentialSource(appID string) string {
	switch {
	case flagAppID != "":
		return "command line"
	case appID != "":
		return "saved account or Qobuz bundle"
	default:
		return "not configured"
	}
}

func writeAuthDiagnostics(output io.Writer, jsonOutput bool, reports []authDiagnostic, diagnosticErr error) error {
	if jsonOutput {
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(reports); err != nil {
			return err
		}
	} else {
		for _, report := range reports {
			fmt.Fprintf(output, "[%s] %s", strings.ToUpper(report.Status), report.Step)
			if report.Detail != "" {
				fmt.Fprintf(output, ": %s", report.Detail)
			}
			fmt.Fprintln(output)
			if report.Suggestion != "" {
				fmt.Fprintf(output, "  Next: %s\n", report.Suggestion)
			}
		}
	}
	return diagnosticErr
}
