package config

type Config struct {
	Debug    bool     `mapstructure:"debug" validate:"required,boolean"`
	Server   Server   `mapstructure:"server"`
	Micropub Micropub `mapstructure:"micropub"`
	Content  Content  `mapstructure:"content"`
	Media    Media    `mapstructure:"media"`
}

type Server struct {
	Micropub    MicropubServer `mapstructure:"micropub"`
	QueryServer QueryServer    `mapstructure:"query_server"`
}

type MicropubServer struct {
	PublicUrl string        `mapstructure:"public_url" validate:"required,url"`
	Server    ServerBinding `mapstructure:"server"`
	Limits    ServerLimits  `mapstructure:"limits"`
}

type QueryServer struct {
	Enabled bool          `mapstructure:"enabled" validate:"boolean"`
	Server  ServerBinding `mapstructure:"server" validate:"required"`
}

type ServerBinding struct {
	Address string `mapstructure:"address" validate:"required,hostname|ip"`
	Port    int    `mapstructure:"port" validate:"required,min=1,max=65535"`
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
	ContentUrl string             `mapstructure:"content_url" validate:"required,url"`
	Pagination Pagination         `mapstructure:"pagination" validate:"required"`
	Strategy   string             `mapstructure:"strategy" validate:"required,oneof=d1"`
	D1         *D1ContentStrategy `mapstructure:"d1" validate:"required_if=Strategy d1"`
}

type Pagination struct {
	Enabled bool `mapstructure:"enabled"`
	PerPage int  `mapstructure:"per_page" validate:"required,min=1"`
}

type D1ContentStrategy struct {
	AccountID   string `mapstructure:"account_id" validate:"required"`
	DatabaseID  string `mapstructure:"database_id" validate:"required"`
	APIToken    string `mapstructure:"api_token" validate:"required"`
	TablePrefix string `mapstructure:"table_prefix" validate:"omitempty,identifier"`
	Endpoint    string `mapstructure:"endpoint" validate:"omitempty,url"`
}

type Media struct {
	MediaUrl string           `mapstructure:"media_url" validate:"required,url"`
	Strategy string           `mapstructure:"strategy" validate:"required,oneof=s3"`
	S3       *S3MediaStrategy `mapstructure:"s3" validate:"required_if=Strategy s3"`
}

type S3MediaStrategy struct {
	AccessKeyId string `mapstructure:"access_key_id" validate:"required"`
	SecretKeyId string `mapstructure:"secret_key_id" validate:"required"`
	Region      string `mapstructure:"region" validate:"omitempty"`
	Bucket      string `mapstructure:"bucket" validate:"required"`
	Endpoint    string `mapstructure:"endpoint" validate:"omitempty,url"`
}
