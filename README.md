# Tokilake: Control your own GPUs like Openrouter 🚀

[](https://www.google.com/search?q=%5Bhttps://opensource.org/licenses/MIT%5D\(https://opensource.org/licenses/MIT\))
**Turn Your Local GPU Clusters into a Private, Globally Distributed AI Cloud.**
[](https://www.google.com/search?q=CONTRIBUTING.md)

Tokilake empowers you to seamlessly access and manage your distributed, privately-owned GPU resources for Large Language Model (LLM) inference. It functions like a commercial cloud service but utilizes **your hardware** under **your complete control**.

Unlock your GPU potential, even if your hardware is behind NATs and firewalls. Tokilake provides a unified, accessible interface to your private AI resources.

## ✨ Why Tokilake?

  * 🌍 **Unlock Hidden GPUs:** Access GPUs behind NAT/Firewalls. Tokiame clients make outbound connections, so your GPU servers don't need public IPs.
  * 🛡️ **Private & Secure:** You own the hardware and control access. Provider authentication between Tokiame and Tokilake is designed to be secure (details on secret management in Tokiame documentation).
  * 🏢 **Unified API Gateway:** A single, OpenAI-compatible endpoint for all your distributed models.
  * 🔗 **Flexible Tokiame Deployment:** Deploy Tokiame agents directly on GPU machines or centrally within your intranet to manage multiple local inference servers.
  * 📉 **Low Latency, High Availability:** Distributed Tokilake server nodes connect users to the nearest resource, with resilience built-in (requires resilient DB).
  * 🚄 **Efficient & Optimized:** Uses gRPC for fast Tokilake-Tokiame communication, focused on LLMs.
  * 🧩 **Simpler Than Orchestrators:** A more streamlined solution for LLM inference access compared to full Kubernetes setups.
  * 🤝 **Share with Your Group:** Easily pool resources within a trusted group.

## ⚙️ How It Works: Tokilake & Tokiame

Tokilake uses a two-component system:

1.  **Tokilake Server:** A smart reverse proxy and API gateway. Multiple Tokilake servers can form a distributed network, sharing state (registered Tokiame clients, namespaces, model mappings) via a central PostgreSQL database. Users connect to the nearest Tokilake node.
2.  **Tokiame Client:** A lightweight agent that connects *outbound* to a Tokilake server.
      * **Deployment:** Tokiame can be deployed on a machine within your private network (intranet). From there, it communicates with your local LLM inference servers (e.g., Ollama, vLLM) that expose an OpenAI-compatible API on that same intranet.
      * **Function:** It registers the available models from these local inference servers with the Tokilake network under a chosen `namespace`. The registration process involves authentication of the Tokiame client to the Tokilake server. It then forwards inference tasks received from a Tokilake server to the appropriate local inference server.

<!-- end list -->

```mermaid
graph LR
    User[End User/Application] -->|OpenAI-compatible API Request| TL_Node1[Tokilake Node 1 e.g. US-East]
    User -->|OpenAI-compatible API Request| TL_Node2[Tokilake Node 2 e.g. EU-West]

    DB[PostgreSQL Cluster] -->|Shared State: Namespaces, Hashed Secrets, Model Mappings| TL_Node1
    DB -->|Shared State: Namespaces, Hashed Secrets, Model Mappings| TL_Node2

    subgraph Your_Private_Network_LAN1 ["Your Private Network / Datacenter LAN"]
        direction LR
        TM_Client1[Tokiame Client - e.g. on a utility server in LAN1]

        subgraph GPU_Host_A_LAN1 ["GPU Host A (LAN1)"]
            InfServA[Inference Server A - e.g. Ollama - OpenAI API]
        end
        subgraph GPU_Host_B_LAN1 ["GPU Host B (LAN1)"]
            InfServB[Inference Server B - e.g. vLLM - OpenAI API]
        end

        TM_Client1 --> |Local HTTP API Call| InfServA
        TM_Client1 --> |Local HTTP API Call| InfServB
    end

    TM_Client1 -.->|Secure Outbound gRPC: Register & Await Tasks| TL_Node1
    %% If you have another separate private network
    %% subgraph Your_Private_Network_LAN2 ["Your Private Network / Office LAN"]
    %%     direction LR
    %%     TM_Client2[Tokiame Client (LAN2)]
    %%     subgraph GPU_Host_C_LAN2 ["GPU Host C (LAN2)"]
    %%         InfServC[Inference Server C]
    %%     end
    %%     TM_Client2 --> |Local HTTP API Call| InfServC
    %% end
    %% TM_Client2 -.->|Secure Outbound gRPC| TL_Node2

    style TM_Client1 fill:#D5F5E3,stroke:#2ECC71
    style InfServA fill:#EBF5FB,stroke:#3498DB
    style InfServB fill:#EBF5FB,stroke:#3498DB

```

**Flow:**

1.  **Setup:** Deploy PostgreSQL, Tokilake Server(s). On your private network(s), deploy LLM inference servers on your GPU machines. Deploy Tokiame Client(s) on a machine within each private network that can reach its local inference servers.
2.  **Registration:** Each Tokiame client authenticates and connects out to a Tokilake server. It registers its `namespace` and the models available from the local inference servers it manages (details typically in Tokiame's local configuration like `model.toml`). This information is stored in PostgreSQL.
3.  **User Request:** User sends an OpenAI-compatible API request (e.g., for `my-lan-1:llama3`) to any Tokilake server.
4.  **Routing & Inference:** Tokilake verifies the request, finds a suitable, connected Tokiame client (`my-lan-1`) that offers the model (`llama3`). The task is securely forwarded to that Tokiame client. Tokiame then makes a local API call to the relevant inference server (e.g., running Llama 3), gets the result, and sends it back through Tokilake to the user.

## 🚀 Getting Started

### Prerequisites

  * **Go**: Version `1.24` or newer (for Tokiame).
  * **Rust & Cargo**: Latest stable (for Tokilake Server).
  * **protoc**: Version `3.19` or newer (ensure compatibility with project's gRPC dependencies).
  * **Git**
  * **PostgreSQL Server** (e.g., v12+) & **psql** client
  * **LLM Inference Server(s)**: Running on your GPU machine(s) and exposing an OpenAI-compatible API accessible within their local network (e.g., [Ollama](https://ollama.com/), [vLLM](https://vllm.ai/)).

### 1\. Set Up PostgreSQL

For simplicity, we recommend running PostgreSQL using Docker:

```bash
docker run -d --name tokilake-postgres --restart unless-stopped --shm-size 512m \
  -e TZ=UTC -e POSTGRES_PASSWORD=your_strong_password \
  -p 5432:5432 postgres:14
```

*Remember to use a strong password and configure your database for your production needs (e.g., user, database name, resilience).*
*Initialize the schema (details will be in `docs/DATABASE_SETUP.md` or similar):*

```bash
# psql -h your_postgres_host -U your_user -d your_database_name < ./sql/schema.sql
```

### 2\. Set Up Your LLM Inference Servers

On each GPU machine, install and run your LLM inference server, ensuring it exposes an OpenAI-compatible API endpoint accessible from where you plan to run Tokiame (e.g., `http://<gpu_host_ip>:11434` for Ollama).

  * **Ollama Example on `192.168.1.101`**:
    ```bash
    # Install from ollama.com on the GPU machine
    ollama pull llama3
    # Ensure Ollama is configured to listen on an IP accessible from your Tokiame host,
    # e.g., OLLAMA_HOST=0.0.0.0 ollama serve (or specific IP like 192.168.1.101)
    ```

### 3\. Build Tokilake & Tokiame

First, clone the project repository:

```bash
git clone https://github.com/Tokimorphling/tokilake # Replace with your actual repo URL
cd tokilake
```

**Build Tokilake Server (Rust):**

```bash
cargo build --release
# The binary will be located at ./target/release/tokilake
```

**Build Tokiame Client (Go):**

```bash
cd tokiame
make # Or: go build -o ../build/bin/tokiame ./cmd/tokiame
# The binary will usually be located at ./build/bin/tokiame or ../build/bin/tokiame
cd .. # Return to the project root
```

*(Note: Pre-compiled binaries or Docker images might be available in the future.)*

### 4\. Run Tokilake Server(s)

Deploy the Tokilake server binary (`target/release/tokilake`) to your chosen server(s). It primarily uses environment variables for configuration (like `DATABASE_URL`) and may have internal defaults for listen addresses.

```bash
export DATABASE_URL="postgres://your_user:your_strong_password@your_postgres_host:5432/your_database_name"
./target/release/tokilake
# Tokilake server will listen on default ports for HTTP API (e.g., 8000)
# and gRPC for Tokiame clients (e.g., 19982).
# Refer to Tokilake documentation for details on changing default ports or other configurations
# (e.g., via a configuration file or other environment variables if supported).
```

*Check the startup logs of the Tokilake server to confirm the IP/hostname and ports it's listening on.*

### 5\. Run Tokiame Client(s)

Deploy the Tokiame client binary (e.g., `build/bin/tokiame`) to a machine that has access to your GPU inference servers running Ollama, vLLM, or SGLang. Ensure that this Tokiame machine has network connectivity to those inference servers

Use the following command structure, providing the required flags:

```bash
./build/bin/tokiame \
  -a "your_tokilake_server_ip_or_domain:19982" \
  -n "my-datacenter-gpus" \
```

  * **`-a, --addr string`**: (Required) The remote gRPC address of your Tokilake server.
  * **`-n, --namespace string`**: (Required) A unique identifier for this Tokiame instance or the group of GPUs it represents. This is used by end-users as part of the model string.
  * **`-l, --api-addr string`**: (Optional) The address on which Tokiame itself will listen for local API requests (e.g., for health checks or local management). Defaults to `localhost:7749`.
  * **`-h, --help`**: Displays help for Tokiame command-line options.

*Always refer to `tokiame --help` for the most up-to-date flags and consult Tokiame's specific documentation for details on provider authentication and model configuration file structure.*

### 🎉 Use Your Private AI Cloud\!

Once both Tokilake server(s) and Tokiame client(s) are running and registered, you can send OpenAI-compatible API requests to your Tokilake server's HTTP endpoint.

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://your_tokilake_server_ip_or_domain:8000/v1", # Tokilake's OpenAI-compatible endpoint
    api_key="YOUR_TOKILAKE_USER_API_KEY" # Currently, API keys are not supported for end-user authentication to Tokilake, but this feature will be added soon. For now, pass any string if your client requires it, or an empty string if allowed. Provider-level authentication (Tokiame to Tokilake) is handled separately as configured in Tokiame.
)

try:
    chat_completion = client.chat.completions.create(
        messages=[{"role": "user", "content": "Explain the Tokilake project in simple terms."}],
        model="my-datacenter-gpus:llama3" # Format: namespace:model_name_as_in_tokiame_config
    )
    print(chat_completion.choices[0].message.content)
except Exception as e:
    print(f"An error occurred: {e}")
```

## 🎯 Simple Usage Scenarios

  * **Solo Developer:** Access your powerful home desktop GPU (running Ollama) from your laptop while traveling. Tokiame runs on a Raspberry Pi (or the desktop itself) on your home network, pointing to your desktop's Ollama API, and connects out to a Tokilake server you host on a cheap VPS.
  * **Team with a LAN:** A small team has several GPU workstations on an office LAN. One Tokiame instance runs on a utility server in that LAN, configured to access OpenAI-compatible APIs on each workstation. The entire team uses a single Tokilake URL (pointing to their self-hosted Tokilake server) to access all these GPUs as a unified resource.
  * **Distributed Research Group:** Collaborators in different labs, each with their own private networks and GPUs, run local Tokiame clients within their respective LANs. They all connect their Tokiame clients to a shared, distributed Tokilake server network for pooled resources and easy cross-lab model access.

## 💡 Additional Use Case: API Forwarder (OpenRouter-like)

Tokilake can function as an API forwarder for Large Language Models via its `/v2/chat/completion` endpoint (or similar, specific endpoint TBD). This allows you to route API requests through a unified Tokilake interface, similar to services like OpenRouter. This feature draws inspiration from the `aichat` implementation. This could be used to centralize access not just to your private Tokiame-connected models but potentially others if Tokilake is extended.

## 🗺️ Current Status & Roadmap

  * ✅ Core functionality for proxying LLM chat completions.
  * ✅ Tokilake server and Tokiame client implementation with gRPC.
  * ✅ PostgreSQL backend for distributed Tokilake nodes and persistent configuration (namespaces, provider authentication info, model registrations).
  * 🔜 **Detailed Documentation:** Comprehensive setup guides, Tokiame model configuration specifics, provider authentication best practices, and future user authentication details.
  * 🔜 **End-User Authentication & Authorization:** Implementing robust authentication (e.g., API keys, OAuth) for users accessing the Tokilake API gateway.
  * 🔜 **Enhanced Model Management & Discovery:** Features within Tokilake/Tokiame for better model tagging, capability reporting, and easier discovery.
  * 🔜 **Support for more inference server backends/types** within Tokiame.
  * 🔜 **Observability:** Enhanced logging, metrics, and tracing for better operational insight.

We are actively developing Tokilake. Contributions and feedback are welcome\!

## 🆚 Comparison to Alternatives

  * **Cloud LLM APIs (OpenAI, Gemini, etc.):** Offer ease of use but can be expensive and raise privacy concerns for sensitive data. Tokilake lets you use your own models on your own hardware.
  * **API Aggregators (OpenRouter, LiteLLM):** Great for accessing *public/commercial* models. Tokilake is your *private* OpenRouter for your *own, non-public* GPUs. LiteLLM is excellent for unifying API calls to various services (including self-hosted models); Tokilake specifically adds the critical layer of making private, NATted GPUs accessible as a unified resource endpoint and manages the registration of these private providers.
  * **DePIN Compute Projects:** Aim to create public, tokenized markets for GPU power. Tokilake is focused on private, trusted networks without the crypto layer.
  * **VPNs/SSH Tunnels/ngrok/Cloudflare Tunnel:** Useful for general remote access but lack the specialized LLM API gateway features, dynamic provider/model registration, (future) load balancing across your private fleet, and centralized management that Tokilake provides for inference tasks.

## 🤝 Contributing

This project is in its early stages, and we welcome your contributions\! If you like what we're building, please consider joining us to make it even better.

  * **Contributing**: See `CONTRIBUTING.md` for details on how to get started with development, report issues, and make pull requests.
  * **Support the Author**: If you'd like to support the ongoing development, you can sponsor the author through cryptocurrency. Your support helps drive progress\! *(Link/address to your sponsorship page/wallet)*

## 🌐 Other Languages

  * [简体中文](README.zh-CN.md) *(Placeholder - create this file if you have a translation)*
  * [Español](https://www.google.com/search?q=README.es.md) *(Placeholder - create this file if you have a translation)*
  * *(Add more languages as they become available)*


-----
## Contributing & Support

This project is in its early stages. If you like it, you are welcome to build it together! Please see `CONTRIBUTING.md` for details on how to get started with development, report issues, and make pull requests.

If you want to support the author, you can sponsor it through cryptocurrency to help it make better progress:



* **Bitcoin:** `bc1pgnls57637tz5ytcf2fmw3f04c35v5dnalg3wycx9qhqvfjdg0ccs62pj63`
* **Ethereum (ETH & ERC-20 Tokens):**  `0x01c264522691cf9142b35aa808a9d518e4bc7028`
* **Solana (SOL & SPL Tokens):** `2KLf23ypzYKfNR7EsSCa8EikUAHdtF1Jr18dgA2WaHxH`