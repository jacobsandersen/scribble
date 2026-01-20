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
	MaxPayloadSize  uint `mapstructure:"max_payload_size" validate:"required"`
	MaxFileSize     uint `mapstructure:"max_file_size" validate:"required"`
	MaxMultipartMem uint `mapstructure:"max_multipart_mem" validate:"required"`
}

type Micropub struct {
	MeUrl         string `mapstructure:"me_url" validate:"required,url"`
	TokenEndpoint string `mapstructure:"token_endpoint" validate:"required,url"`
}

type Content struct {
	Strategy string              `mapstructure:"strategy" validate:"required,oneof=git sql d1"`
	Git      *GitContentStrategy `mapstructure:"git" validate:"required_if=Strategy git"`
	SQL      *SQLContentStrategy `mapstructure:"sql" validate:"required_if=Strategy sql"`
	D1       *D1ContentStrategy  `mapstructure:"d1" validate:"required_if=Strategy d1"`
}

type GitContentStrategy struct {
	Repository string                 `mapstructure:"repository" validate:"required,url"`
	Path       string                 `mapstructure:"path" validate:"required,localpath"`
	PublicUrl  string                 `mapstructure:"public_url" validate:"required,url"`
	Auth       GitContentStrategyAuth `mapstructure:"auth"`
}

type GitContentStrategyAuth struct {
	Method string                `mapstructure:"method" validate:"required,oneof=plain ssh"`
	Plain  *UsernamePasswordAuth `mapstructure:"plain" validate:"required_if=Method plain"`
	Ssh    *SshKeyAuth           `mapstructure:"ssh" validate:"required_if=Method ssh"`
}

type SQLContentStrategy struct {
	Driver      string  `mapstructure:"driver" validate:"required,oneof=postgres mysql"`
	DSN         string  `mapstructure:"dsn" validate:"required"`
	PublicUrl   string  `mapstructure:"public_url" validate:"required,url"`
	TablePrefix *string `mapstructure:"table_prefix" validate:"omitempty,identifier"`
}

type D1ContentStrategy struct {
	AccountID   string  `mapstructure:"account_id" validate:"required"`
	DatabaseID  string  `mapstructure:"database_id" validate:"required"`
	APIToken    string  `mapstructure:"api_token" validate:"required"`
	PublicUrl   string  `mapstructure:"public_url" validate:"required,url"`
	TablePrefix *string `mapstructure:"table_prefix" validate:"omitempty,identifier"`
	Endpoint    string  `mapstructure:"endpoint" validate:"omitempty,url"`
}

type UsernamePasswordAuth struct {
	Username string `mapstructure:"username" validate:"required"`
	Password string `mapstructure:"password" validate:"required"`
}

type SshKeyAuth struct {
	Username           string `mapstructure:"username" validate:"required"`
	PrivateKeyFilePath string `mapstructure:"private_key_file_path" validate:"required,file"`
	Passphrase         string `mapstructure:"passphrase" validate:"required"`
}

type Media struct {
	Strategy string           `mapstructure:"strategy" validate:"required,oneof=s3"`
	S3       *S3MediaStrategy `mapstructure:"s3" validate:"required_if=Strategy s3"`
}

type S3MediaStrategy struct {
	AccessKeyId    string `mapstructure:"access_key_id" validate:"required"`
	SecretKeyId    string `mapstructure:"secret_key_id" validate:"required"`
	Region         string `mapstructure:"region" validate:"omitempty"`
	Bucket         string `mapstructure:"bucket" validate:"required"`
	Endpoint       string `mapstructure:"endpoint" validate:"omitempty,url"`
	ForcePathStyle bool   `mapstructure:"force_path_style"`
	DisableSSL     bool   `mapstructure:"disable_ssl"`
	Prefix         string `mapstructure:"prefix"`
	PublicUrl      string `mapstructure:"public_url" validate:"omitempty,url"`
}
