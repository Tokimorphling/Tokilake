# 开发与贡献

## 目录

- [本地构建](#本地构建)
  - [环境配置](#环境配置)
  - [编译流程](#编译流程)
  - [运行说明](#运行说明)
- [Docker 构建](#docker-构建)
  - [环境配置](#环境配置-1)
  - [编译流程](#编译流程-1)
  - [运行说明](#运行说明-1)

## 本地构建

### 环境配置

你需要一个 golang 与 yarn 开发环境

#### 直接安装

golang 官方安装指南：https://go.dev/doc/install \
yarn 官方安装指南：https://yarnpkg.com/getting-started/install

#### 通过 conda/mamba 安装 （没错它不只能管理 python）

如果你已有[conda](https://docs.conda.io/projects/conda/en/latest/user-guide/install/index.html)或者[mamba](https://github.com/conda-forge/miniforge)的经验，也可将其用于 golang 环境管理：

```bash
conda create -n goenv go yarn
# mamba create -n goenv go yarn # 如果你使用 mamba
```

### 编译流程

项目根目录已经提供了本地构建的 makefile

```bash
# cd one-hub
# 确保你已经启动了开发环境，比如conda activate goenv
make all
# 更多 make 命令，详见makefile
```

编译成功之后你应当能够在项目根目录找到 `dist` 与 `web/build` 两个文件夹。

### 运行说明

运行

```bash
$ ./dist/one-api -h
Usage of ./dist/one-api:
  -config string
        specify the config.yaml path (default "config.yaml")
  -export
        Exports prices to a JSON file.
  -help
        print help and exit
  -log-dir string
        specify the log directory
  -port int
        the listening port
  -version
        print version and exit
```

根据[使用方法](/use/index)进行具体的项目配置。

## Docker 构建

### 环境配置

你需要 docker 环境，列出下列文档作为安装参考，任选其一即可：

- MirrorZ Help，此为校园网 cernet 镜像站：https://help.mirrors.cernet.edu.cn/docker-ce/
- docker 官方安装文档：https://docs.docker.com/engine/install/

### 编译流程

项目根目录已经提供了 Dockerfile，可以分别构建 Tokilake Hub 和 Tokiame worker 镜像。

```bash
docker build --target tokilake -t tokilake:dev .
docker build --target tokiame -t tokiame:dev .
```

编译成功后，运行

```bash
docker images | grep 'tokilake:dev\|tokiame:dev'
```

你应当能找到刚刚编译的镜像，注意与项目官方镜像区分名称。

### 运行说明

Hub 本地运行示例：

```bash
docker run -d \
  --name tokilake-dev \
  --restart unless-stopped \
  -p 19981:19981 \
  -e PORT=19981 \
  -e SERVER_ADDRESS="http://localhost:19981" \
  -e USER_TOKEN_SECRET="$(openssl rand -hex 32)" \
  -e SESSION_SECRET="$(openssl rand -hex 32)" \
  -v tokilake-dev-data:/data \
  tokilake:dev
```

Tokiame 本地运行需要准备配置文件，具体参考 [Tokilake 与 Tokiame](../deployment/tokilake-tokiame.md)。
