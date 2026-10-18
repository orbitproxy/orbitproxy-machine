# orbitproxy

[orbitproxy](https://orbitproxy.cc) 的命令行客户端。它运行在你的服务旁边，主动连接到 orbitproxy 云网关，把发往你的公网地址的流量转发到本机服务。客户端只发起出站连接，不在本机监听端口。

SDK 客户端见 [orbitproxy-go](https://github.com/orbitproxy/orbitproxy-go)。

## 安装

在 [控制台的安装指引](https://dashboard.app.orbitproxy.cloud/install) 下载对应操作系统的客户端。

## 使用

```bash
# 用账户的 authtoken 注册本机。不要使用 sudo。
orbitproxy authtoken <token>

# 在浏览器中确认登录并注册本机。没有终端时只打印确认地址。
orbitproxy login

# 前台运行，关闭终端即停止。终端中显示状态面板，日志写入文件。
orbitproxy run

# 后台运行。
orbitproxy run --background

# 查看本机的客户端状态，不启动客户端。
orbitproxy status

# 停止后台运行的客户端。不影响前台运行和系统服务。
orbitproxy stop

# 登出：使本机密钥失效，停止后台客户端，删除本地凭据。
orbitproxy logout
```

创建和关闭分享：

```bash
# 创建 API 网关分享并输出公网地址。命令不启动客户端，需要客户端处于运行状态流量才会到达本机。
orbitproxy proxy add --type api_gateway --protocol https --port 8080

# 列出指向本机的分享。
orbitproxy list

# 按分享 ID 或本地端口关闭分享，域名保留。
orbitproxy close <proxyId|port>
```

系统服务在开机时启动。注册服务需要管理员权限，进程以当前用户运行，读取当前用户的 `~/.orbitproxy`：

```bash
sudo orbitproxy service install
sudo orbitproxy service status
sudo orbitproxy service stop
sudo orbitproxy service start
sudo orbitproxy service restart
sudo orbitproxy service uninstall
```

`sudo` 找不到客户端时，改用绝对路径：

```bash
sudo "$(command -v orbitproxy)" service install
```

清除本机数据：

```bash
# 停止客户端，移除系统服务，删除除程序外的本地数据。
orbitproxy reset

# 在 reset 的基础上同时删除客户端程序。该操作不可撤销。
orbitproxy delete
```

查看版本：

```bash
orbitproxy version
```

## 本地数据

数据目录默认为 `~/.orbitproxy`，可用 `--workdir` 指定其他目录。每台机器一个子目录：

```text
<root>/<machineKey>/orbitproxy.yml
<root>/<machineKey>/logs/orbitproxy.log
<root>/<machineKey>/orbitproxy.lock
```

`orbitproxy run` 与系统服务共用这把锁，同一台机器不会同时运行两个客户端。配置不会被复制到 `/etc`。

## 从源码构建

需要 Go 1.25.8 或更高版本。

```bash
make build CONTROL_PLANE_URL=https://cp.app.orbitproxy.cloud TLS_VERIFY=on
```

产物是当前目录下的 `orbitproxy`。

`VERSION` 是客户端版本号，构建时注入到二进制中。发布前用 `scripts/bump-version.sh 1.2.3` 更新版本号。

不带这些参数直接执行 `go build` 时，二进制中的版本号为空，客户端无法注册。

本地开发时连接本机的网关：

```bash
make build-local
```

`build-local` 关闭 TLS 证书校验，只用于本地开发。

## 许可证

[MIT](LICENSE)
