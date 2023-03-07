module github.com/NectGmbH/dnslb

go 1.12

require github.com/sirupsen/logrus v1.6.0

require (
	github.com/Jille/raft-grpc-leader-rpc v1.1.0
	github.com/Jille/raft-grpc-transport v1.1.1
	github.com/Jille/raftadmin v1.2.0
	github.com/NectGmbH/dns v0.0.0-20210621142925-9f6fd7ecf455
	github.com/NectGmbH/health v0.0.0-20210426094827-f2591eccf724
	github.com/OneOfOne/xxhash v1.2.8
	github.com/coreos/etcd v3.3.25+incompatible // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/hashicorp/raft v1.3.1
	github.com/hashicorp/raft-boltdb v0.0.0-20210422161416-485fa74b0b01
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/motemen/go-loghttp v0.0.0-20170804080138-974ac5ceac27
	github.com/namsral/flag v1.7.4-pre
	github.com/prometheus/client_golang v1.10.0
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.7.1
	go.etcd.io/etcd v3.3.25+incompatible
	go.uber.org/zap v1.16.0 // indirect
	google.golang.org/grpc v1.52.0
	google.golang.org/grpc/examples v0.0.0-20230306234545-3292193519c3 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.0.0-20190313235455-40a48860b5ab // indirect
	k8s.io/apimachinery v0.15.7
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog v0.3.3 // indirect
	k8s.io/kube-openapi v0.0.0-20190709113604-33be087ad058 // indirect
	k8s.io/utils v0.0.0-20190712204705-3dccf664f023 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace github.com/go-openapi/strfmt => github.com/pkavajin/strfmt v0.19.3-0.20190719095505-eef5ad02c4fa
