package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "wireflow",
	Short: "wireflow is a tool for creating fast and secure wireguard proxies",
	Long:  `wireflow is a tool for creating fast and secure wireguard proxies. It allows you to create a wireguard interface and manage it easily.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func init() {
	//	cobra.OnInitialize(initConfig)
	//
	//	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobra.yaml)")
	//	rootCmd.PersistentFlags().StringP("author", "a", "YOUR NAME", "author name for copyright attribution")
	//	rootCmd.PersistentFlags().StringVarP(&userLicense, "license", "l", "", "name of license for the project")
	//	rootCmd.PersistentFlags().Bool("viper", true, "use Viper for configuration")
	//	viper.BindPFlag("author", rootCmd.PersistentFlags().Lookup("author"))
	//	viper.BindPFlag("useViper", rootCmd.PersistentFlags().Lookup("viper"))
	//	viper.SetDefault("author", "NAME HERE <EMAIL ADDRESS>")
	//	viper.SetDefault("license", "apache")
	//
	rootCmd.AddCommand(up())
	rootCmd.AddCommand(loginCmd())
	rootCmd.AddCommand(managementCmd())
	rootCmd.AddCommand(signalingCmd())
	rootCmd.AddCommand(turnCmd())
	rootCmd.AddCommand(stop())
	rootCmd.AddCommand(status())
}

//
//func initConfig() {
//	if cfgFile != "" {
//		// Use config file from the flag.
//		viper.SetConfigFile(cfgFile)
//	} else {
//		// Find home directory.
//		home, err := os.UserHomeDir()
//		cobra.CheckErr(err)
//
//		// Search config in home directory with name ".cobra" (without extension).
//		viper.AddConfigPath(home)
//		viper.SetConfigType("yaml")
//		viper.SetConfigName(".cobra")
//	}
//
//	viper.AutomaticEnv()
//
//	if err := viper.ReadInConfig(); err == nil {
//		fmt.Println("Using config file:", viper.ConfigFileUsed())
//	}
//}
