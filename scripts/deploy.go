//go:build ignore

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	fmt.Println("txtscape Railway deployment")
	fmt.Println("==========================")
	fmt.Println()

	// Preflight checks
	if _, err := exec.LookPath("railway"); err != nil {
		fatal("railway CLI not found. Install: npm i -g @railway/cli")
	}

	out, err := output("railway", "whoami")
	if err != nil {
		fatal("Not logged in. Run: railway login")
	}
	fmt.Printf("Logged in as: %s\n", strings.TrimSpace(out))

	// Step 1: Project — skip if already linked
	step("1/4", "Project")
	if _, err := output("railway", "status"); err == nil {
		fmt.Println("  → Already linked to a Railway project, skipping init")
	} else {
		confirm("Create a new Railway project named 'txtscape'?")
		mustRun("railway", "init", "--name", "txtscape")
	}

	// Step 2: Postgres — skip if DATABASE_URL is already set
	step("2/4", "PostgreSQL")
	if dbURL, err := output("railway", "variable", "list"); err == nil && strings.Contains(dbURL, "DATABASE_URL") {
		fmt.Println("  → DATABASE_URL already set, skipping database creation")
	} else {
		confirm("Add a PostgreSQL database to this project?")
		mustRun("railway", "add", "--database", "postgres")
	}

	// Step 3: Custom domain
	step("3/4", "Custom domain")
	confirm("Add custom domain txtscape.com? (safe to re-add if it already exists)")
	mustRun("railway", "domain", "txtscape.com")

	// Step 4: Deploy
	step("4/4", "Deploy")
	confirm("Deploy to Railway? (uses Dockerfile + railway.json config)")
	mustRun("railway", "up", "--detach")

	fmt.Println()
	fmt.Println("=== Deployment started ===")
	fmt.Println()
	fmt.Println("Remaining manual steps:")
	fmt.Println("  1. Add DNS record: CNAME txtscape.com → <service>.up.railway.app")
	fmt.Println("     (Railway prints the target above)")
	fmt.Println("  2. Wait for SSL provisioning (~1-5 min after DNS propagates)")
	fmt.Println("  3. Verify: curl https://txtscape.com/index.txt")
	fmt.Println("  4. Monitor: railway logs")
}

func step(num, name string) {
	fmt.Printf("\n[%s] %s\n", num, name)
}

func confirm(msg string) {
	fmt.Printf("  %s [y/N] ", msg)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer != "y" && answer != "yes" {
		fmt.Println("  Skipped.")
		return
	}
}

func mustRun(prog string, args ...string) {
	fmt.Printf("  $ %s %s\n", prog, strings.Join(args, " "))
	cmd := exec.Command(prog, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n  FAILED: %v\n", err)
		fmt.Fprintln(os.Stderr, "  Fix the issue and re-run. This script is idempotent.")
		os.Exit(1)
	}
}

func output(prog string, args ...string) (string, error) {
	cmd := exec.Command(prog, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func fatal(msg string) {
	fmt.Fprintf(os.Stderr, "ERROR: %s\n", msg)
	os.Exit(1)
}
