#!/bin/bash
expireTime="87600h"
serviceName="admission-registry"
serviceNamespace="default"
[ $(which cfssl | wc -l) -eq 0 ] && wget https://pkg.cfssl.org/R1.2/cfssl_linux-amd64 && chmod +x cfssl_linux-amd64 && mv cfssl_linux-amd64 /usr/local/bin/cfssl
[ $(which cfssljson | wc -l) -eq 0 ] && wget https://pkg.cfssl.org/R1.2/cfssljson_linux-amd64 && chmod +x cfssljson_linux-amd64 && cfssljson_linux-amd64 /usr/local/bin/cfssljson

cat > ca-config.json <<EOF
{
  "signing": {
    "default": {
      "expiry": "${expireTime}"
    },
    "profiles": {
      "server": {
        "usages": ["signing", "key encipherment", "server auth", "client auth"],
        "expiry": "${expireTime}"
      }
    }
  }
}
EOF

cat > ca-csr.json <<EOF
{
    "CN": "kubernetes",
    "key": {
        "algo": "rsa",
        "size": 2048
    },
    "names": [
        {
            "C": "CN",
            "L": "BeiJing",
            "ST": "BeiJing",
            "O": "k8s",
            "OU": "System"
        }
    ]
}
EOF

cfssl gencert -initca ca-csr.json | cfssljson -bare ca

cat > server-csr.json <<EOF
{
  "CN": "admission",
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
        "C": "CN",
        "L": "BeiJing",
        "ST": "BeiJing",
        "O": "k8s",
        "OU": "System"
    }
  ]
}
EOF

# -hostname 的值，格式为  {service-name}.{service-namespace}.svc，其中 service-name 代表你 webhook 的 Service 名字，service-namespace 代表你 webhook 的命名空间。
#cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json -hostname="${serviceName}.${serviceNamespace}.svc" -profile=server server-csr.json | cfssljson -bare server
cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json -hostname=admission-registry.default.svc -profile=server server-csr.json | cfssljson -bare server

#kubectl delete secret admission-registry-tls
#kubectl create secret tls admission-registry-tls \
#        --key=server-key.pem \
#        --cert=server.pem

echo "Use this follow output to replace CA_BUNDLE in mutating | validating webhook yaml"
cat ca.pem | base64
