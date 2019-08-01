module github.com/NectGmbH/dns

go 1.12

require (
	github.com/NectGmbH/autodns/client/zone_tasks v0.0.0-20190718115733-8c783ffac8f52e60712b0ac760d18f916f24712f
	github.com/NectGmbH/autodns/models v0.0.0-20190718115733-d00dd94a9cb7
	github.com/OneOfOne/xxhash v1.2.5
	github.com/go-openapi/errors v0.19.2
	github.com/go-openapi/runtime v0.19.3
	github.com/go-openapi/strfmt v0.19.2
	github.com/mitchellh/mapstructure v1.1.2
	github.com/motemen/go-loghttp v0.0.0-20170804080138-974ac5ceac27
	github.com/motemen/go-nuts v0.0.0-20180315145558-42c35bdb11c2 // indirect
	github.com/sirupsen/logrus v1.4.2
	gopkg.in/yaml.v3 v3.0.0-20190709130402-674ba3eaed22
)

replace github.com/go-openapi/strfmt => github.com/pkavajin/strfmt v0.19.3-0.20190719095505-eef5ad02c4fa
