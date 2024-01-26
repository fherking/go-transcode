package config

import (
	"encoding/xml"
	"fmt"
	"os"
	"path"

	"github.com/rs/zerolog/log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Root struct {
	Debug   bool
	PProf   bool
	CfgFile string
}

func (Root) Init(cmd *cobra.Command) error {
	cmd.PersistentFlags().BoolP("debug", "d", false, "enable debug mode")
	if err := viper.BindPFlag("debug", cmd.PersistentFlags().Lookup("debug")); err != nil {
		return err
	}

	cmd.PersistentFlags().Bool("pprof", false, "enable pprof endpoint available at /debug/pprof")
	if err := viper.BindPFlag("pprof", cmd.PersistentFlags().Lookup("pprof")); err != nil {
		return err
	}

	cmd.PersistentFlags().String("config", "", "configuration file path")
	if err := viper.BindPFlag("config", cmd.PersistentFlags().Lookup("config")); err != nil {
		return err
	}

	return nil
}

func (s *Root) Set() {
	s.Debug = viper.GetBool("debug")
	s.PProf = viper.GetBool("pprof")
	s.CfgFile = viper.GetString("config")
}

type VideoProfile struct {
	Width   int `mapstructure:"width"`
	Height  int `mapstructure:"height"`
	Bitrate int `mapstructure:"bitrate"` // in kilobytes
}

type AudioProfile struct {
	Bitrate int `mapstructure:"bitrate"` // in kilobytes
}

type VOD struct {
	MediaDir       string                  `mapstructure:"media-dir"`
	TranscodeDir   string                  `mapstructure:"transcode-dir"`
	VideoProfiles  map[string]VideoProfile `mapstructure:"video-profiles"`
	VideoKeyframes bool                    `mapstructure:"video-keyframes"`
	AudioProfile   AudioProfile            `mapstructure:"audio-profile"`
	Cache          bool                    `mapstructure:"cache"`
	CacheDir       string                  `mapstructure:"cache-dir"`
	FFmpegBinary   string                  `mapstructure:"ffmpeg-binary"`
	FFprobeBinary  string                  `mapstructure:"ffprobe-binary"`
}

type Enigma2 struct {
	WebifUrl  string `mapstructure:"webif-url"`
	StreamUrl string `mapstructure:"stream-url"`
	Bouquet   string
	Reference string
}

type ServiceList struct {
	XMLName     xml.Name  `xml:"e2servicelist"`
	ServiceList []Service `xml:"e2service"`
}

type Service struct {
	XMLName   xml.Name `xml:"e2service"`
	Name      string   `xml:"e2servicename"`
	Reference string   `xml:"e2servicereference"`
}

type Server struct {
	Cert   string
	Key    string
	Bind   string
	Static string
	Proxy  bool
	CORS   bool

	BaseDir  string            `yaml:"basedir,omitempty"`
	Streams  map[string]string `yaml:"streams"`
	Profiles string            `yaml:"profiles,omitempty"`

	Enigma2 Enigma2

	Vod      VOD
	HlsProxy map[string]string
}

func (Server) Init(cmd *cobra.Command) error {
	cmd.PersistentFlags().String("bind", "127.0.0.1:8080", "address/port/socket to serve neko")
	if err := viper.BindPFlag("bind", cmd.PersistentFlags().Lookup("bind")); err != nil {
		return err
	}

	cmd.PersistentFlags().String("cert", "", "path to the SSL cert used to secure the neko server")
	if err := viper.BindPFlag("cert", cmd.PersistentFlags().Lookup("cert")); err != nil {
		return err
	}

	cmd.PersistentFlags().String("key", "", "path to the SSL key used to secure the neko server")
	if err := viper.BindPFlag("key", cmd.PersistentFlags().Lookup("key")); err != nil {
		return err
	}

	cmd.PersistentFlags().String("static", "", "path to neko client files to serve")
	if err := viper.BindPFlag("static", cmd.PersistentFlags().Lookup("static")); err != nil {
		return err
	}

	cmd.PersistentFlags().Bool("proxy", false, "allow reverse proxies: X-Forwarded-For headers will be used to determine the client IP")
	if err := viper.BindPFlag("proxy", cmd.PersistentFlags().Lookup("proxy")); err != nil {
		return err
	}

	cmd.PersistentFlags().Bool("cors", false, "enable CORS")
	if err := viper.BindPFlag("cors", cmd.PersistentFlags().Lookup("cors")); err != nil {
		return err
	}

	cmd.PersistentFlags().String("basedir", "", "base directory for assets and profiles")
	if err := viper.BindPFlag("basedir", cmd.PersistentFlags().Lookup("basedir")); err != nil {
		return err
	}

	cmd.PersistentFlags().String("profiles", "", "hardware encoding profiles to load for ffmpeg (default, nvidia)")
	if err := viper.BindPFlag("profiles", cmd.PersistentFlags().Lookup("profiles")); err != nil {
		return err
	}

	return nil
}

func (s *Server) Set() {
	s.Cert = viper.GetString("cert")
	s.Key = viper.GetString("key")
	s.Bind = viper.GetString("bind")
	s.Static = viper.GetString("static")
	s.Proxy = viper.GetBool("proxy")
	s.CORS = viper.GetBool("cors")

	s.BaseDir = viper.GetString("basedir")
	if s.BaseDir == "" {
		if _, err := os.Stat("/etc/transcode"); os.IsNotExist(err) {
			cwd, _ := os.Getwd()
			s.BaseDir = cwd
		} else {
			s.BaseDir = "/etc/transcode"
		}
	}

	s.Profiles = viper.GetString("profiles")
	if s.Profiles == "" {
		// TODO: issue #5
		s.Profiles = fmt.Sprintf("%s/profiles", s.BaseDir)
	}
	s.Streams = viper.GetStringMapString("streams")

	//
	// VOD
	//
	if err := viper.UnmarshalKey("vod", &s.Vod); err != nil {
		panic(err)
	}

	// defaults

	if s.Vod.TranscodeDir == "" {
		var err error
		s.Vod.TranscodeDir, err = os.MkdirTemp(os.TempDir(), "go-transcode-vod")
		if err != nil {
			panic(err)
		}
	} else {
		err := os.MkdirAll(s.Vod.TranscodeDir, 0755)
		if err != nil {
			panic(err)
		}
	}

	if len(s.Vod.VideoProfiles) == 0 {
		panic("specify at least one VOD video profile")
	}

	if s.Vod.Cache && s.Vod.CacheDir != "" {
		err := os.MkdirAll(s.Vod.CacheDir, 0755)
		if err != nil {
			panic(err)
		}
	}

	if s.Vod.FFmpegBinary == "" {
		s.Vod.FFmpegBinary = "ffmpeg"
	}

	if s.Vod.FFprobeBinary == "" {
		s.Vod.FFprobeBinary = "ffprobe"
	}

	//
	// HLS PROXY
	//
	s.HlsProxy = viper.GetStringMapString("hls-proxy")

	//
	// Enigma2
	//
	if err := viper.UnmarshalKey("enigma2", &s.Enigma2); err != nil {
		panic(err)
	}

	if s.Enigma2.WebifUrl != "" {
		enigma2Streams, err := parseEnigma2Config(s.Enigma2)
		if err != nil {
			panic(err)
		}

		for k, v := range enigma2Streams {
			s.Streams[k] = v
		}

		log.Info().Msgf("loaded %d streams from Enigma2", len(enigma2Streams))
	}
}

func (s *Server) AbsPath(elem ...string) string {
	// prepend base path
	elem = append([]string{s.BaseDir}, elem...)
	return path.Join(elem...)
}
