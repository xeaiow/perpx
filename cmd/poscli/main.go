package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/yourname/poscli/internal/config"
)

var (
	flagConfigPath string
)

func main() {
	root := &cobra.Command{
		Use:   "poscli",
		Short: "Multi-exchange perpetual position manager (TUI)",
		Long: `poscli is a TUI CLI for managing perpetual futures positions across
Binance, OKX, Bybit, Bitget, Gate, and Zoomex. API keys are stored in
config.toml encrypted with AES-256-GCM, unlocked by a master password.`,
	}

	defaultPath := defaultConfigPath()
	root.PersistentFlags().StringVarP(&flagConfigPath, "config", "c", defaultPath, "path to config.toml")

	root.AddCommand(
		newInitCmd(),
		newAddCmd(),
		newVerifyCmd(),
		newRotateCmd(),
		newRunCmd(),
	)

	if err := root.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func defaultConfigPath() string {
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".config", "poscli", "config.toml")
	}
	return "config.toml"
}

// readPassword 從 TTY 讀密碼，不回顯。
// 回傳的 []byte 由呼叫端負責 Zeroize。
func readPassword(prompt string) ([]byte, error) {
	fmt.Fprint(os.Stderr, prompt)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("read password: %w", err)
	}
	return pw, nil
}

// readPasswordConfirm 讀兩次比對。
func readPasswordConfirm(prompt string) ([]byte, error) {
	pw1, err := readPassword(prompt)
	if err != nil {
		return nil, err
	}
	pw2, err := readPassword("Confirm: ")
	if err != nil {
		config.Zeroize(pw1)
		return nil, err
	}
	defer config.Zeroize(pw2)
	if string(pw1) != string(pw2) {
		config.Zeroize(pw1)
		return nil, errors.New("passwords do not match")
	}
	if len(pw1) < 8 {
		config.Zeroize(pw1)
		return nil, errors.New("password too short (minimum 8 characters)")
	}
	return pw1, nil
}

// readLine 從 stdin 讀一行明文，給非機密輸入用（API key 不是真的密碼但
// 也算機密，但常見作法是讓使用者貼上後再加密）。
func readLine(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// ---- init ----

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create a new config.toml with master password",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat(flagConfigPath); err == nil {
				return fmt.Errorf("config already exists at %s (use 'add' to update, or delete it first)", flagConfigPath)
			}
			if err := os.MkdirAll(filepath.Dir(flagConfigPath), 0700); err != nil {
				return fmt.Errorf("mkdir config dir: %w", err)
			}

			password, err := readPasswordConfirm("Master password: ")
			if err != nil {
				return err
			}
			defer config.Zeroize(password)

			salt, err := config.NewSalt()
			if err != nil {
				return err
			}
			kdf := config.DefaultKDFParams()
			key := config.DeriveKey(password, salt, kdf)
			defer config.Zeroize(key)

			cfg := &config.Config{
				Security: config.Security{
					Salt:      base64Encode(salt),
					KDF:       "argon2id",
					KDFParams: kdf,
				},
				Runtime:   config.Runtime{HTTPTimeoutSec: 10},
				Exchanges: map[string]*config.ExchangeConfig{},
			}

			fmt.Fprintln(os.Stderr, "\nEnter API credentials for each exchange (leave key blank to skip):")
			for _, name := range config.AllExchanges {
				ec, err := promptExchangeCreds(key, name)
				if err != nil {
					return err
				}
				cfg.Exchanges[string(name)] = ec
			}

			if err := config.Save(flagConfigPath, cfg); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "\nConfig written to %s (mode 0600)\n", flagConfigPath)
			return nil
		},
	}
}

func promptExchangeCreds(key []byte, name config.ExchangeName) (*config.ExchangeConfig, error) {
	fmt.Fprintf(os.Stderr, "\n[%s]\n", name)
	apiKey, err := readLine("  API key (blank to skip): ")
	if err != nil {
		return nil, err
	}
	if apiKey == "" {
		return &config.ExchangeConfig{Enabled: false}, nil
	}
	apiSecret, err := readLine("  API secret: ")
	if err != nil {
		return nil, err
	}
	if apiSecret == "" {
		return nil, fmt.Errorf("%s: API secret required when key is provided", name)
	}

	ec := &config.ExchangeConfig{Enabled: true}
	encKey, err := config.Encrypt(key, []byte(apiKey))
	if err != nil {
		return nil, err
	}
	ec.APIKey = encKey
	encSec, err := config.Encrypt(key, []byte(apiSecret))
	if err != nil {
		return nil, err
	}
	ec.APISecret = encSec

	if requiresPP(name) {
		pp, err := readLine("  Passphrase: ")
		if err != nil {
			return nil, err
		}
		if pp == "" {
			return nil, fmt.Errorf("%s: passphrase required", name)
		}
		encPP, err := config.Encrypt(key, []byte(pp))
		if err != nil {
			return nil, err
		}
		ec.Passphrase = encPP
	}
	return ec, nil
}

func requiresPP(name config.ExchangeName) bool {
	return name == config.OKX || name == config.Bitget
}

// ---- add (stub for M1 完成度，先放這裡，等後續完整實作) ----

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <exchange>",
		Short: "Add or update credentials for an exchange",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("add: not yet implemented")
		},
	}
}

// ---- verify ----

func newVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Verify config can be decrypted (does not call any exchange API)",
		RunE: func(cmd *cobra.Command, args []string) error {
			password, err := readPassword("Master password: ")
			if err != nil {
				return err
			}
			defer config.Zeroize(password)

			result, err := config.Load(flagConfigPath, password)
			if err != nil {
				return err
			}
			defer result.Zeroize()

			fmt.Println("Config OK.")
			fmt.Printf("Enabled exchanges (%d):\n", len(result.Credentials))
			for _, name := range config.AllExchanges {
				c, ok := result.Credentials[name]
				if !ok {
					continue
				}
				fmt.Printf("  - %-8s api_key_len=%d secret_len=%d", name, len(c.APIKey), len(c.APISecret))
				if c.Passphrase != nil {
					fmt.Printf(" passphrase_len=%d", len(c.Passphrase))
				}
				fmt.Println()
			}
			return nil
		},
	}
}

// ---- rotate-password (stub) ----

func newRotateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rotate-password",
		Short: "Re-encrypt all credentials with a new master password",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("rotate-password: not yet implemented")
		},
	}
}

// ---- run (TUI, stub) ----

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Start the TUI (default command)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("run: TUI not yet implemented (M5)")
		},
	}
}

func base64Encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
