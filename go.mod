module github.com/Tokimorphling/Tokilake

replace one-api => ./

replace github.com/Tokimorphling/Tokilake/tokilake-core => ./tokilake-core

replace github.com/Tokimorphling/Tokilake/tokiame => ./tokiame

// +heroku goVersion go1.18
go 1.25.0

require (
	cloud.google.com/go/iam v1.9.0
	github.com/ThinkInAIXYZ/go-mcp v0.2.26
	github.com/Tokimorphling/Tokilake/tokiame v0.0.0-20260425121306-dc5e0650b8b6
	github.com/Tokimorphling/Tokilake/tokilake-core v0.12.1
	github.com/aliyun/aliyun-oss-go-sdk v3.0.2+incompatible
	github.com/anknown/ahocorasick v0.0.0-20190904063843-d75dbd5169c0
	github.com/aws/aws-sdk-go-v2 v1.41.7
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.10
	github.com/aws/aws-sdk-go-v2/credentials v1.19.16
	github.com/aws/aws-sdk-go-v2/service/s3 v1.100.1
	github.com/aws/smithy-go v1.25.1
	github.com/bwmarrin/snowflake v0.3.0
	github.com/bytedance/gopkg v0.1.4
	github.com/coocood/freecache v1.2.7
	github.com/coreos/go-oidc/v3 v3.18.0
	github.com/eko/gocache/lib/v4 v4.2.3
	github.com/eko/gocache/store/freecache/v4 v4.2.4
	github.com/eko/gocache/store/redis/v4 v4.2.6
	github.com/gin-contrib/cors v1.7.7
	github.com/gin-contrib/gzip v1.2.6
	github.com/gin-contrib/sessions v1.1.0
	github.com/gin-contrib/static v1.1.6
	github.com/gin-gonic/gin v1.12.0
	github.com/go-co-op/gocron/v2 v2.21.1
	github.com/go-gormigrate/gormigrate/v2 v2.1.5
	github.com/go-playground/validator/v10 v10.30.2
	github.com/go-webauthn/webauthn v0.17.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/gomarkdown/markdown v0.0.0-20260417124207-7d523f7318df
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/mitchellh/mapstructure v1.5.0
	github.com/pires/go-proxyproto v0.12.0
	github.com/pkoukk/tiktoken-go v0.1.8
	github.com/prometheus/client_golang v1.23.2
	github.com/redis/go-redis/v9 v9.19.0
	github.com/samber/lo v1.53.0
	github.com/shopspring/decimal v1.4.0
	github.com/smartwalle/alipay/v3 v3.2.29
	github.com/spf13/viper v1.21.0
	github.com/sqids/sqids-go v0.4.1
	github.com/stretchr/testify v1.11.1
	github.com/stripe/stripe-go/v80 v80.2.1
	github.com/vmihailenco/msgpack/v5 v5.4.1
	github.com/wechatpay-apiv3/wechatpay-go v0.2.21
	github.com/wneessen/go-mail v0.7.2
	go.uber.org/zap v1.28.0
	golang.org/x/crypto v0.50.0
	golang.org/x/image v0.39.0
	golang.org/x/oauth2 v0.36.0
	golang.org/x/sync v0.20.0
	google.golang.org/api v0.277.0
	google.golang.org/grpc v1.80.0
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
	gorm.io/driver/mysql v1.6.0
	gorm.io/driver/postgres v1.6.0
	gorm.io/driver/sqlite v1.6.0
	gorm.io/gorm v1.31.1
	one-api v0.0.0-00010101000000-000000000000
)

require (
	cloud.google.com/go/auth v0.20.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/anknown/darts v0.0.0-20151216065714-83ff685239e6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.23 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bytedance/sonic/loader v0.5.1 // indirect
	github.com/cloudwego/base64x v0.1.7 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.10.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.1 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/go-webauthn/x v0.2.3 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.15 // indirect
	github.com/googleapis/gax-go/v2 v2.22.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/orcaman/concurrent-map/v2 v2.0.1 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.59.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/smartwalle/ncrypto v1.0.4 // indirect
	github.com/smartwalle/ngx v1.1.0 // indirect
	github.com/smartwalle/nsign v1.0.9 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tinylib/msgp v1.6.4 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	go.mongodb.org/mongo-driver/v2 v2.6.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.68.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.68.0 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260427160629-7cedc36a6bc4 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260427160629-7cedc36a6bc4 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

require (
	github.com/PaulSonOfLars/gotgbot/v2 v2.0.0-rc.34
	github.com/bytedance/sonic v1.15.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dlclark/regexp2 v1.12.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/gin-contrib/sse v1.1.1 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-sql-driver/mysql v1.10.0 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/gorilla/context v1.1.2 // indirect
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/gorilla/sessions v1.4.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.9.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/mattn/go-sqlite3 v1.14.44 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.1 // indirect
	github.com/xtaci/smux v1.5.57
	golang.org/x/arch v0.26.0 // indirect
	golang.org/x/net v0.53.0
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gorm.io/datatypes v1.2.7
)
