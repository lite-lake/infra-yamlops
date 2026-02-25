# 服务器安装 Docker CE 指南

仅适用于 ubuntu 22.04

## 国内

```
curl -fsSL https://get.docker.com | sudo sh -s docker --mirror Aliyun
```

## 国外

```
curl -fsSL https://get.docker.com | sh
```

## 备用 手动配置清华源并用 apt 安装

```
# 1. 删除旧的 Docker 源配置（如果存在）
sudo rm -f /etc/apt/sources.list.d/docker.list

# 2. 安装依赖工具
sudo apt update
sudo apt install -y ca-certificates curl

# 3. 添加 Docker 的 GPG 密钥（使用清华源）
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://mirrors.tuna.tsinghua.edu.cn/docker-ce/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

# 4. 添加 Docker APT 源（清华源）
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://mirrors.tuna.tsinghua.edu.cn/docker-ce/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list

# 5. 更新并安装 Docker
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# 6. （可选）将当前用户加入 docker 组
sudo usermod -aG docker $USER
newgrp docker
```
