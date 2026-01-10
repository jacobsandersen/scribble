package config

type Config struct {
	Debug    bool     `mapstructure:"debug"`
	Server   Server   `mapstructure:"server"`
	Micropub Micropub `mapstructure:"micropub"`
	Content  Content  `mapstructure:"content"`
	Media    Media    `mapstructure:"media"`
}

type Server struct {
	Address   string       `mapstructure:"address" validate:"required,hostname|ip"`
	Port      int          `mapstructure:"port" validate:"required,min=1,max=65535"`
	PublicUrl string       `mapstructure:"public_url" validate:"required,url"`
	Limits    ServerLimits `mapstructure:"limits"`
}

type ServerLimits struct {
	MaxPayloadSize uint `mapstructure:"max_payload_size" validate:"required"`
	MaxFileSize    uint `mapstructure:"max_file_size" validate:"required"`
}

type Micropub struct {
	MeUrl         string `mapstructure:"me_url" validate:"required,url"`
	TokenEndpoint string `mapstructure:"token_endpoint" validate:"required,url"`
}

type Content struct {
	Strategy string              `mapstructure:"strategy" validate:"required,oneof=git"`
	Git      *GitContentStrategy `mapstructure:"git" validate:"required_if=Strategy git"`
}

type GitContentStrategy struct {
	Repository string                 `mapstructure:"repository" validate:"required,url"`
	Path       string                 `mapstructure:"path" validate:"required,dirpath"`
	PublicUrl  string                 `mapstructure:"public_url" validate:"required,url"`
	LocalPath  string                 `mapstructure:"local_path" validate:"required,abspath"`
	Auth       GitContentStrategyAuth `mapstructure:"auth"`
}

type GitContentStrategyAuth struct {
	Method string                `mapstructure:"method" validate:"required,oneof=plain ssh"`
	Plain  *UsernamePasswordAuth `mapstructure:"plain" validate:"required_if=Method plain"`
	Ssh    *SshKeyAuth           `mapstructure:"ssh" validate:"required_if=Method ssh"`
}

type UsernamePasswordAuth struct {
	Username string `mapstructure:"username" validate:"required"`
	Password string `mapstructure:"password" validate:"required"`
}

type SshKeyAuth struct {
	PrivateKeyFilePath string `mapstructure:"private_key_file_path" validate:"required,file"`
	Passphrase         string `mapstructure:"passphrase" validate:"required"`
}

type Media struct {
	Strategy string           `mapstructure:"strategy" validate:"required,oneof=s3"`
	S3       *S3MediaStrategy `mapstructure:"s3" validate:"required_if=Strategy s3"`
}

type S3MediaStrategy struct {
	AccessKeyId string `mapstructure:"access_key_id" validate:"required"`
	SecretKeyId string `mapstructure:"secret_key_id" validate:"required"`
	Region      string `mapstructure:"region" validate:"required"`
	Bucket      string `mapstructure:"bucket" validate:"required"`
}
