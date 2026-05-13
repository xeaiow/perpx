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

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/yourname/poscli/internal/config"
	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/closelog"
	"github.com/yourname/poscli/internal/ui"

	// 啟用 adapter 註冊
	_ "github.com/yourname/poscli/internal/exchange/binance"
	_ "github.com/yourname/poscli/internal/exchange/bitget"
	_ "github.com/yourname/poscli/internal/exchange/bybit"
	_ "github.com/yourname/poscli/internal/exchange/gate"
	_ "github.com/yourname/poscli/internal/exchange/okx"
	_ "github.com/yourname/poscli/internal/exchange/zoomex"
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

// ---- run ----
//
// Load config、prompt password、建立 registry、啟動 TUI。

func newRunCmd() *cobra.Command {
	var flagLogFile string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start the TUI (default command)",
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

			reg, err := exchange.NewRegistry(result)
			if err != nil {
				return err
			}
			if len(reg) == 0 {
				return errors.New("no exchange adapters enabled in config.toml")
			}

			exs := make(map[string]exchange.Exchange, len(reg))
			for name, ex := range reg {
				exs[string(name)] = ex
			}

			closelog.Init(resolveLogFile(flagLogFile, result.Config.Runtime.LogFile))
			defer closelog.Sync()

			prog := tea.NewProgram(ui.NewFromMap(exs))
			_, err = prog.Run()
			return err
		},
	}
	cmd.Flags().StringVar(&flagLogFile, "log-file", "",
		"close-position log path (overrides $POSCLI_LOG_FILE and runtime.log_file in config.toml)")
	return cmd
}

func base64Encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// resolveLogFile 依優先級鏈算出最終 log path：
//
//	1. --log-file flag      （明確指定、最高優先）
//	2. $POSCLI_LOG_FILE     （臨時 override，不污染 config）
//	3. runtime.log_file     （config.toml 持久設定）
//	4. closelog.DefaultPath ()（fallback：~/.config/poscli/close.log）
func resolveLogFile(flag, fromConfig string) string {
	if flag != "" {
		return flag
	}
	if env := os.Getenv(closelog.EnvOverride); env != "" {
		return env
	}
	if fromConfig != "" {
		return fromConfig
	}
	return closelog.DefaultPath()
}
