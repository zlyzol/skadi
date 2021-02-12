package config

import (
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Configuration  for chain service
type Configuration struct {
	Scheme			string 			   `json:"scheme" mapstructure:"scheme"`
	Host     		string 			   `json:"host" mapstructure:"host"`
	ListenPort      int                `json:"listen_port" mapstructure:"listen_port"`
	ShutdownTimeout time.Duration      `json:"shutdown_timeout" mapstructure:"shutdown_timeout"`
	ReadTimeout     time.Duration      `json:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout    time.Duration      `json:"write_timeout" mapstructure:"write_timeout"`
	LogLevel        string             `json:"log_level" mapstructure:"log_level"`
	Pretty			bool			   `json:"pretty" mapstructure:"pretty"`
	Mongo           MongoConfiguration `json:"mongo" mapstructure:"mongo"`
	Exchanges       ExchangesConfiguration  `json:"exchanges" mapstructure:"exchanges"`
	Accounts        AccountsConfiguration  `json:"accounts" mapstructure:"accounts"`
}
type MongoConfiguration struct {
	Host     string `json:"host" mapstructure:"host"`
	Port     int    `json:"port" mapstructure:"port"`
	Database string `json:"database" mapstructure:"database"`
}
type ExchangesConfiguration map[string] ExchangeConfiguration
type ExchangeConfiguration struct {
	Type		string `json:"type" mapstructure:"type"`
	AccountName	string `json:"account_name" mapstructure:"account_name"`
	Parameters	map[string]string
}
type AccountsConfiguration map[string] AccountConfiguration
type AccountConfiguration struct {
	Name	string
	Chains	AccountDetailChainConfiguration
}
type AccountDetailChainConfiguration map[string] AccountDetailConfiguration
type AccountDetailConfiguration struct {
	KeystoreOrApiKey	string `json:"keystore_or_api_key" mapstructure:"keystore_or_api"`
	PasswordOrSecretKey string `json:"password_or_secret_key" mapstructure:"password_or_secret_key"`
}
func applyDefaultConfig() {
	viper.SetDefault("read_timeout", "30s")
	viper.SetDefault("write_timeout", "30s")
	viper.SetDefault("shutdown_timeout", "30s")
	viper.SetDefault("scheme", "http")
	viper.SetDefault("host", "localhost")
	viper.SetDefault("listen_port", "8082")
	viper.SetDefault("log_level", "info") // debug
	viper.SetDefault("pretty", "false")
	viper.SetDefault("mongo.host", "skip") // localhost
	viper.SetDefault("mongo.port", "27017")
	viper.SetDefault("mongo.database", "test")
	viper.SetDefault("exchanges", ExchangesConfiguration{
		"binance_dex": ExchangeConfiguration{ Type: "binance-dex", AccountName: "key_files" }, 
		"thorchain": ExchangeConfiguration{ Type: "thorchain", AccountName: "key_files", Parameters: map[string]string{ "seed_url": "chaosnet-seed.thorchain.info" }}, 
		"binance": ExchangeConfiguration{ Type: "binance", AccountName: "binance_api" }, 
//		"bitmax": ExchangeConfiguration{ Type: "bitmax", AccountName: "bitmax_api" },
	})
	viper.SetDefault("accounts", AccountsConfiguration{
		"key_files": AccountConfiguration { 
			Name: "key_files",
			Chains: AccountDetailChainConfiguration {
				"BNB": AccountDetailConfiguration{ KeystoreOrApiKey: "key.json", PasswordOrSecretKey: "BepPassword01@" },
				"BTC": AccountDetailConfiguration{ KeystoreOrApiKey: "btc.dat", PasswordOrSecretKey: "btcpwd" },
				"ETH": AccountDetailConfiguration{ KeystoreOrApiKey: "eth.json", PasswordOrSecretKey: "ethpwd" },
			},
		},
		"binance_api": AccountConfiguration {
			Name: "binance_api",
			Chains: AccountDetailChainConfiguration {
				"ALL": AccountDetailConfiguration{ KeystoreOrApiKey: "zfzoFyLGeVBIMNAR2NsRXgoXIB1V4QGDKXk1eTdNZ909VtVPvx8dP27e9lPyzX7y", PasswordOrSecretKey: "O0TCrJIchcQvU0diHdmGNf0UxfvxDsOH9skTF0DuO1vDSNhx0vmDNTiMTbdB3ztV" },
			},
		},
		"bitmax_api": AccountConfiguration {
			Name: "bitmax_api",
			Chains: AccountDetailChainConfiguration {
				"ALL": AccountDetailConfiguration{ KeystoreOrApiKey: "x", PasswordOrSecretKey: "y" },
			},
		},
	})
	// BTC bitcoin
	// https://www.thepolyglotdeveloper.com/2018/03/create-sign-bitcoin-transactions-golang/
	// https://www.thepolyglotdeveloper.com/2018/03/create-bitcoin-hardware-wallet-golang-raspberry-pi-zero/
}
func LoadConfiguration(file string) (*Configuration, error) {
	applyDefaultConfig()
	var cfg Configuration
	viper.SetConfigName(strings.TrimRight(path.Base(file), ".json"))
	viper.AddConfigPath(".")
	viper.AddConfigPath(filepath.Dir(file))
	if err := viper.ReadInConfig(); nil != err {
		return nil, errors.Wrap(err, "failed to read from config file")
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	if err := viper.Unmarshal(&cfg); nil != err {
		return nil, errors.Wrap(err, "failed to unmarshal")
	}
	return &cfg, nil
}
