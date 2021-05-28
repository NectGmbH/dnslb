module github.com/NectGmbH/dnslb

go 1.12

require github.com/sirupsen/logrus v1.6.0

require (
	github.com/Jille/raft-grpc-leader-rpc v1.1.0 // indirect
	github.com/Jille/raft-grpc-transport v1.1.1 // indirect
	github.com/Jille/raftadmin v1.2.0 // indirect
	github.com/NectGmbH/dns v0.0.0-20210430124002-d803a4da61f2
	github.com/NectGmbH/health v0.0.0-20210426094827-f2591eccf724
	github.com/OneOfOne/xxhash v1.2.8
	github.com/coreos/etcd v3.3.25+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/hashicorp/raft v1.3.1 // indirect
	github.com/hashicorp/raft-boltdb v0.0.0-20210422161416-485fa74b0b01 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/motemen/go-loghttp v0.0.0-20170804080138-974ac5ceac27
	github.com/namsral/flag v1.7.4-pre
	github.com/prometheus/client_golang v1.10.0
	go.etcd.io/etcd v3.3.25+incompatible
	go.etcd.io/etcd/client/v3 v3.5.0-alpha.0
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b // indirect
	golang.org/x/oauth2 v0.0.0-20210427180440-81ed05c6b58c // indirect
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba // indirect
	google.golang.org/grpc v1.32.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/api v0.0.0-20190313235455-40a48860b5ab // indirect
	k8s.io/apimachinery v0.0.0-20190313205120-d7deff9243b1
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog v0.3.3 // indirect
	k8s.io/kube-openapi v0.0.0-20190709113604-33be087ad058 // indirect
	k8s.io/utils v0.0.0-20190712204705-3dccf664f023 // indirect
)

replace github.com/go-openapi/strfmt => github.com/pkavajin/strfmt v0.19.3-0.20190719095505-eef5ad02c4fa
