#!/bin/bash

# 生成自签名 HTTPS 证书脚本
# 用于 Oppama 项目的本地开发和测试

set -e

# 配置变量
CERT_DIR="${1:-./certs}"
DAYS_VALID=365
COUNTRY="CN"
STATE="Beijing"
LOCALITY="Beijing"
ORGANIZATION="Oppama"
COMMON_NAME="localhost"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "╔════════════════════════════════════════════════════════════╗"
echo "║          生成 Oppama 自签名 HTTPS 证书                      ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# 检查 OpenSSL 是否安装
if ! command -v openssl &> /dev/null; then
    echo -e "${RED}错误：未找到 OpenSSL，请先安装 OpenSSL${NC}"
    echo "macOS: brew install openssl"
    echo "Linux: apt-get install openssl 或 yum install openssl"
    exit 1
fi

# 创建证书目录
if [ ! -d "$CERT_DIR" ]; then
    mkdir -p "$CERT_DIR"
    echo -e "${GREEN}✓ 创建证书目录：${CERT_DIR}${NC}"
fi

# 生成配置文件
cat > "${CERT_DIR}/openssl.cnf" <<EOF
[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn
x509_extensions = v3_req

[dn]
C = ${COUNTRY}
ST = ${STATE}
L = ${LOCALITY}
O = ${ORGANIZATION}
CN = ${COMMON_NAME}

[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = *.localhost
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

echo -e "${GREEN}✓ 创建 OpenSSL 配置文件${NC}"

# 生成私钥和证书
echo ""
echo "正在生成 RSA 私钥和自签名证书..."
openssl req -x509 -nodes -days ${DAYS_VALID} -newkey rsa:2048 \
    -keyout "${CERT_DIR}/server.key" \
    -out "${CERT_DIR}/server.crt" \
    -config "${CERT_DIR}/openssl.cnf" \
    -extensions v3_req

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ 生成私钥：${CERT_DIR}/server.key${NC}"
    echo -e "${GREEN}✓ 生成证书：${CERT_DIR}/server.crt${NC}"
else
    echo -e "${RED}✗ 证书生成失败${NC}"
    exit 1
fi

# 设置文件权限
chmod 600 "${CERT_DIR}/server.key"
chmod 644 "${CERT_DIR}/server.crt"
echo -e "${GREEN}✓ 设置文件权限（私钥 600，证书 644）${NC}"

# 显示证书信息
echo ""
echo "╔════════════════════════════════════════════════════════════╗"
echo "║                    证书信息                                ║"
echo "╠════════════════════════════════════════════════════════════╣"
echo -e "║  证书目录：${CERT_DIR}"
echo "║  有效期：${DAYS_VALID} 天"
echo "║  域名：localhost, *.localhost, 127.0.0.1"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# 显示使用说明
echo "╔════════════════════════════════════════════════════════════╗"
echo "║                  使用指南                                  ║"
echo "╠════════════════════════════════════════════════════════════╣"
echo "║  1. 修改 config.yaml 配置：                                 ║"
echo "║     enable_https: true                                     ║"
echo "║     cert_file: ${CERT_DIR}/server.crt"
echo "║     key_file: ${CERT_DIR}/server.key"
echo "║                                                            ║"
echo "║  2. 启动服务：                                              ║"
echo "║     ./oppama -config config.yaml                           ║"
echo "║                                                            ║"
echo "║  3. 访问地址：                                              ║"
echo "║     https://localhost:8080/admin                          ║"
echo "║                                                            ║"
echo "║  ⚠️  注意：                                                 ║"
echo "║     - 浏览器会提示证书不受信任，这是正常的                 ║"
echo "║     - 点击\"继续访问\"或\"接受风险\"即可                     ║"
echo "║     - 生产环境请使用正式 CA 签发的证书                     ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# 在 macOS 上提示是否添加到钥匙串
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "💡 macOS 用户可以将证书添加到钥匙串以消除浏览器警告："
    echo "   sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ${CERT_DIR}/server.crt"
    echo ""
fi

echo -e "${GREEN}✓ 证书生成完成！${NC}"
