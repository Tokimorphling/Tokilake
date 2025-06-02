# Tokilake：您的私有分布式AI云平台 🚀

[](https://www.google.com/search?q=%5Bhttps://opensource.org/licenses/MIT%5D\(https://opensource.org/licenses/MIT\))
[](https://www.google.com/search?q=CONTRIBUTING.md)
**将本地GPU集群转化为私有、全球分布的AI云平台**

Tokilake让您能够无缝访问和管理分布式私有GPU资源，用于大语言模型（LLM）推理。它提供类似商业云服务的体验，但完全使用**您的硬件**并在**您的完全控制**下运行。

即使您的硬件位于NAT和防火墙之后，也能释放GPU的全部潜力。Tokilake为您的私有AI资源提供统一的访问接口。

## ✨ 为什么选择Tokilake？

* 🌍 **解锁闲置GPU**：访问位于NAT/防火墙后的GPU。Tokiame客户端采用出站连接，因此您的GPU服务器无需公网IP。
* 🛡️ **私有且安全**：您拥有硬件所有权和访问控制权。Tokiame与Tokilake之间的提供者认证机制安全可靠（详见Tokiame文档中的密钥管理说明）。
* 🏢 **统一API网关**：为所有分布式模型提供OpenAI兼容的统一接入点。
* 🔗 **灵活的Tokiame部署**：可直接在GPU机器上部署Tokiame代理，或在企业内网集中管理多个本地推理服务器。
* 📉 **低延迟，高可用**：分布式Tokilake服务节点将用户连接到最近的资源，并具备内置弹性（需配合弹性数据库）。
* 🚄 **高效优化**：使用gRPC实现Tokilake-Tokiame高速通信，专注LLM支持（计划支持TTS）。
* 🧩 **比编排器更简单**：相比完整的Kubernetes方案，提供更简洁的LLM推理访问方案。
* 🤝 **团队资源共享**：轻松在可信赖的团队中共享资源。

## ⚙️ 工作原理：Tokilake与Tokiame

Tokilake采用双组件系统：

1. **Tokilake服务器**：智能反向代理和API网关。多个Tokilake服务器可组成分布式网络，通过中央PostgreSQL数据库共享状态（注册的Tokiame客户端、命名空间、模型映射）。用户连接最近的Tokilake节点。
2. **Tokiame客户端**：轻量级代理程序，通过**出站连接**对接Tokilake服务器。
   - **部署方式**：Tokiame可部署在私有网络（内网）中的机器上，通过与本地LLM推理服务器（如Ollama、vLLM）通信，这些服务器需在内网暴露OpenAI兼容API。
   - **功能**：它将本地推理服务器的可用模型注册到Tokilake网络的指定`命名空间`下。注册过程包含Tokiame客户端对Tokilake服务器的认证，并负责将Tokilake下发的推理任务转发到对应的本地推理服务器。

```mermaid
graph LR
    User[终端用户/应用] -->|OpenAI兼容API请求| TL_Node1[Tokilake节点1 如美东]
    User -->|OpenAI兼容API请求| TL_Node2[Tokilake节点2 如欧西]

    DB[PostgreSQL集群] -->|共享状态：命名空间、哈希密钥、模型映射| TL_Node1
    DB -->|共享状态：命名空间、哈希密钥、模型映射| TL_Node2

    subgraph 您的私有网络_LAN1 ["您的私有网络/数据中心LAN"]
        direction LR
        TM_Client1[Tokiame客户端 - 如LAN1中的工具服务器]

        subgraph GPU主机A_LAN1 ["GPU主机A (LAN1)"]
            InfServA[推理服务器A - 如Ollama - OpenAI API]
        end
        subgraph GPU主机B_LAN1 ["GPU主机B (LAN1)"]
            InfServB[推理服务器B - 如vLLM - OpenAI API]
        end

        TM_Client1 --> |本地HTTP API调用| InfServA
        TM_Client1 --> |本地HTTP API调用| InfServB
    end

    TM_Client1 -.->|安全出站gRPC：注册并等待任务| TL_Node1
    %% 若存在其他独立私有网络
    %% subgraph 您的私有网络_LAN2 ["您的私有网络/办公室LAN"]
    %%     direction LR
    %%     TM_Client2[Tokiame客户端 (LAN2)]
    %%     subgraph GPU主机C_LAN2 ["GPU主机C (LAN2)"]
    %%         InfServC[推理服务器C]
    %%     end
    %%     TM_Client2 --> |本地HTTP API调用| InfServC
    %% end
    %% TM_Client2 -.->|安全出站gRPC| TL_Node2

    style TM_Client1 fill:#D5F5E3,stroke:#2ECC71
    style InfServA fill:#EBF5FB,stroke:#3498DB
    style InfServB fill:#EBF5FB,stroke:#3498DB
```

**工作流程：**

1. **初始化**：部署PostgreSQL和Tokilake服务器。在私有网络中，GPU机器上部署LLM推理服务器。在每个私有网络中部署Tokiame客户端（需能访问本地推理服务器）。
2. **注册**：每个Tokiame客户端认证并连接到Tokilake服务器，注册其`命名空间`及其管理的本地推理服务器可用模型（具体配置在Tokiame的`model.toml`等文件中）。这些信息存储在PostgreSQL中。
3. **用户请求**：用户向任意Tokilake服务器发送OpenAI兼容API请求（如请求`my-lan-1:llama3`）。
4. **路由与推理**：Tokilake验证请求，找到提供该模型（`llama3`）且已连接的Tokiame客户端（`my-lan-1`）。任务被安全转发至该客户端，Tokiame调用本地推理服务器API（如运行Llama 3的实例），获取结果后通过Tokilake返回给用户。

## 🚀 快速入门

### 先决条件

* **Go**：1.21或更新版本（用于Tokiame）
* **Rust & Cargo**：最新稳定版（用于Tokilake服务器）
* **protoc**：3.19或更新版本（需与项目gRPC依赖兼容）
* **Git**
* **PostgreSQL服务器**（v12+）及**psql**客户端
* **LLM推理服务器**：在GPU机器上运行并暴露OpenAI兼容API（如[Ollama](https://ollama.com/)、[vLLM](https://vllm.ai/)）

### 1. 设置PostgreSQL

推荐使用Docker简化部署：

```bash
docker run -d --name tokilake-postgres --restart unless-stopped --shm-size 512m \
  -e TZ=UTC -e POSTGRES_PASSWORD=your_strong_password \
  -p 5432:5432 postgres:14
```

*请使用强密码并根据生产需求配置数据库（用户、数据库名、弹性等）*
*初始化数据库架构（详见`docs/DATABASE_SETUP.md`）：*

```bash
# psql -h your_postgres_host -U your_user -d your_database_name < ./sql/schema.sql
```

### 2. 设置LLM推理服务器

在每个GPU主机上安装运行推理服务器，确保Tokiame运行位置可访问其OpenAI兼容API端点（如Ollama的`http://<gpu_host_ip>:11434`）。

* **Ollama示例（IP：192.168.1.101）**：
  ```bash
  # 在GPU主机上从ollama.com安装
  ollama pull llama3
  # 确保Ollama监听Tokiame主机可访问的IP
  # 例如：OLLAMA_HOST=0.0.0.0 ollama serve (或指定IP如192.168.1.101)
  ```

### 3. 构建Tokilake & Tokiame

克隆项目仓库：

```bash
git clone https://github.com/Tokimorphling/tokilake # 替换为实际仓库URL
cd tokilake
```

**构建Tokilake服务器（Rust）：**

```bash
cargo build --release
# 二进制文件位于 ./target/release/tokilake
```

**构建Tokiame客户端（Go）：**

```bash
cd tokiame
make # 或：go build -o ../build/bin/tokiame ./cmd/tokiame
# 二进制文件通常位于 ./build/bin/tokiame 或 ../build/bin/tokiame
cd .. # 返回项目根目录
```

*(注：未来可能提供预编译二进制文件或Docker镜像)*

### 4. 运行Tokilake服务器

将Tokilake服务器二进制文件（`target/release/tokilake`）部署到选定服务器。主要通过环境变量配置（如`DATABASE_URL`），监听地址可能有内部默认值。

```bash
export DATABASE_URL="postgres://your_user:your_strong_password@your_postgres_host:5432/your_database_name"
./target/release/tokilake
# Tokilake服务器默认监听HTTP API端口（如8000）
# 和Tokiame客户端的gRPC端口（如19982）
# 请参考Tokilake文档修改默认端口或其他配置
# （如通过配置文件或其他支持的环境变量）
```

*检查Tokilake服务器启动日志确认监听的IP/主机名和端口*

### 5. 运行Tokiame客户端

将Tokiame客户端二进制文件（如`build/bin/tokiame`）部署到能访问GPU服务器的服务器上。运行Tokiame的机器需能访问这些推理服务器。

使用以下命令结构并提供必要参数：

```bash
./build/bin/tokiame \
  -a "your_tokilake_server_ip_or_domain:19982" \
  -n "my-datacenter-gpus" \
```

* **`-a, --addr string`**：（必需）Tokilake服务器的远程gRPC地址
* **`-n, --namespace string`**：（必需）此Tokiame实例或GPU组的唯一标识符，用户将以此作为模型名称前缀
* **`-l, --api-addr string`**：（可选）Tokiame监听本地API请求的地址（如健康检查），默认`localhost:7749`
* **`-h, --help`**：显示Tokiame命令行帮助

*请通过`tokiame --help`获取最新参数，并参考Tokiame文档了解提供者认证和模型配置文件细节*

### 🎉 使用您的私有AI云平台！

当Tokilake服务器和Tokiame客户端均运行并注册后，即可向Tokilake服务器的HTTP端点发送OpenAI兼容API请求。

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://your_tokilake_server_ip_or_domain:8000/v1", # Tokilake的OpenAI兼容端点
    api_key="YOUR_TOKILAKE_USER_API_KEY" # 当前版本暂不支持终端用户API密钥认证，此功能将在后续版本添加。如需占位可传任意字符串或空值。提供者级认证（Tokiame到Tokilake）在Tokiame配置中单独处理。
)

try:
    chat_completion = client.chat.completions.create(
        messages=[{"role": "user", "content": "用简单语言解释Tokilake项目"}],
        model="my-datacenter-gpus:llama3" # 格式：命名空间:Tokiame配置中的模型名
    )
    print(chat_completion.choices[0].message.content)
except Exception as e:
    print(f"发生错误: {e}")
```

## 🎯 典型使用场景

* **独立开发者**：旅行时通过笔记本电脑访问家中高性能桌面GPU（运行Ollama）。Tokiame运行在家用网络的树莓派（或桌面本身）上，连接到桌面Ollama API，并出站连接到部署在廉价VPS的Tokilake服务器。
* **团队局域网**：小型团队在办公局域网拥有多台GPU工作站。在局域网工具服务器上运行一个Tokiame实例，配置为访问每台工作站的OpenAI兼容API。全团队使用单一Tokilake URL（指向自托管的Tokilake服务器）统一访问所有GPU资源。
* **分布式研究组**：不同实验室的协作者在各自私有网络和GPU上运行本地Tokiame客户端，所有客户端连接到共享的分布式Tokilake服务器网络，实现资源池化和跨实验室模型访问。

## 💡 其他用例：API转发器（OpenRouter模式）

Tokilake可通过其`/v2/chat/completion`端点（或类似特定端点）作为LLM的API转发器。该功能受`aichat`实现启发，可用于集中访问公共的API模型，未来可能扩展支持其他模型。
## 🗺️ 当前状态与路线图

* ✅ LLM聊天补全代理核心功能
* ✅ 基于gRPC的Tokilake服务器和Tokiame客户端实现
* ✅ 支持分布式Tokilake节点和持久化配置的PostgreSQL后端（命名空间、提供者认证信息、模型注册）
* 🔜 **详细文档**：完整设置指南、Tokiame模型配置规范、提供者认证最佳实践及未来用户认证细节
* 🔜 **终端用户认证与授权**：为访问Tokilake API网关的用户实现健壮认证（API密钥、OAuth等）
* 🔜 **增强模型管理与发现**：改进模型标签、能力报告和更便捷的发现功能
* 🔜 **支持文本转语音（TTS）模型**：（视强大开源方案可用性及需求而定）
* 🔜 **支持更多推理服务器后端类型**
* 🔜 **可观测性**：增强日志、指标和追踪以提高运维洞察力

我们正在积极开发Tokilake。欢迎贡献和反馈！

## 🆚 与替代方案对比

* **云LLM API（OpenAI、Gemini等）**：易用但成本高昂，敏感数据存在隐私顾虑。Tokilake让您在自有硬件上使用私有模型。
* **API聚合器（OpenRouter、LiteLLM）**：适用于访问*公共/商业*模型。Tokilake是专属于*您私有非公开*GPU的*私有*OpenRouter。LiteLLM擅长统一访问各类服务（含自托管模型）；Tokilake专门增加了将私有NAT隔离GPU作为统一资源端点访问的关键层，并管理这些私有提供者的注册。
* **DePIN计算项目**：旨在创建公开的代币化GPU市场。Tokilake专注于无需加密层的私有可信网络。
* **VPN/SSH隧道/ngrok/Cloudflare隧道**：适用于通用远程访问，但缺乏Tokilake专为推理任务设计的LLM API网关特性、动态提供者/模型注册、（未来）私有资源池负载均衡和集中管理功能。

## 🤝 贡献指南

本项目处于早期阶段，欢迎您的贡献！如果您认可我们的理念，请加入我们共同完善。

* **贡献代码**：请参阅`CONTRIBUTING.md`了解开发流程、问题报告和PR提交规范。
* **支持作者**：若希望支持持续开发，可通过加密货币赞助作者。您的支持将推动项目发展！*(赞助页面/钱包地址链接)*

## 🌐 多语言支持

* [简体中文](https://www.google.com/search?q=README.zh-CN.md) *(占位符 - 若有翻译请创建此文件)*
* [Español](https://www.google.com/search?q=README.es.md) *(占位符 - 若有翻译请创建此文件)*
* *(添加更多语言版本)*

-----
## 贡献与支持

本项目处于早期阶段。如果您感兴趣，欢迎共同建设！请参阅`CONTRIBUTING.md`了解开发流程、问题报告和PR提交规范。

若希望支持作者，可通过加密货币赞助加速项目发展：

* **比特币(Bitcoin):** `bc1pgnls57637tz5ytcf2fmw3f04c35v5dnalg3wycx9qhqvfjdg0ccs62pj63`
* **以太坊(Ethereum, ETH & ERC-20代币):** `0x01c264522691cf9142b35aa808a9d518e4bc7028`
* **Solana (SOL & SPL代币):** `2KLf23ypzYKfNR7EsSCa8EikUAHdtF1Jr18dgA2WaHxH`