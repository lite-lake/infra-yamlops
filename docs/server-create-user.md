# 服务器创建 ubuntu 用户指南

仅适用于 ubuntu 22.04

```
useradd -m -s /bin/bash ubuntu
echo "ubuntu:这里替换为密码" | chpasswd
usermod -aG sudo ubuntu
cat > /etc/sudoers.d/ubuntu <<EOF
ubuntu ALL=(ALL:ALL) NOPASSWD: ALL
EOF
chmod 440 /etc/sudoers.d/ubuntu
```
