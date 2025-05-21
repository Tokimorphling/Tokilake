# Tokilake

**Turn Your Local GPU Clusters into a Private, Globally Distributed AI Cloud**

[](https://opensource.org/licenses/MIT)

Tokilake empowers you to seamlessly access and manage your distributed, privately-owned GPU resources for Large Language Model (LLM) inference, just like a commercial cloud service, but under your complete control. Support for Text-to-Speech (TTS) models is planned for the future.

## The Challenge: Unlocking Private GPU Potential

The rise of powerful open-source LLMs is exciting. However, leveraging your own or your friends' GPU hardware often comes with hurdles:

  * **Remote Access Nightmares:** Running inference servers (like llama.cpp, SGLang, vLLM) on home or office machines is common. But accessing them securely and conveniently from anywhere, especially when they lack a public IP, is a major pain point. Exposing public IPs directly is a security risk.
  * **Stranded & Underutilized GPUs:** Significant GPU power often sits idle behind NATs and firewalls, inaccessible for your projects or to share within a trusted group.
  * **Managing Multiple Endpoints:** Juggling different IP addresses and ports for various models running on disparate machines becomes cumbersome.

## Our Solution: Tokilake & Tokiame

Tokilake provides a robust, two-component system to create your own private AI inference cloud:

  * **Tokilake Server:** A smart reverse proxy that acts as a unified API gateway to all your registered GPU resources. Crucially, Tokilake servers can be deployed as a distributed network, sharing state via a central PostgreSQL database. This allows users and GPU providers to connect to the nearest Tokilake node for optimal latency and high availability.
  * **Tokiame Client:** A lightweight agent you deploy on your GPU machines. Tokiame securely connects *outbound* to a Tokilake instance (so **no public IP is needed for your GPUs**), registers the available models, and forwards inference tasks from Tokilake.

Think of Tokilake as your personal, self-hostable OpenRouter, designed for your private GPU fleet.

## Key Features

  * ✨ **Access GPUs Behind NAT/Firewalls:** Tokiame's outbound connection model means your GPU machines don't need a public IP or complex port forwarding.
  * 🌐 **Distributed Tokilake Network:** Deploy multiple Tokilake server nodes globally. They share configuration (like registered Tokiame clients and their namespaces) via a common PostgreSQL database cluster.
      * 📉 **Reduced Latency:** Users and GPU providers connect to the nearest Tokilake node.
      * 🛡️ **Enhanced Availability:** The system remains operational even if individual Tokilake nodes fail (requires a resilient database setup).
      * 🔑 **Centralized & Secure Configuration:** Tokiame registrations and provider namespaces are stored in the database. **Importantly, for API secrets, Tokilake stores only their cryptographically secure hashes, not the raw secrets themselves, enhancing security.**
  * 🚀 **Optimized for LLMs (TTS Planned):** Currently focused on delivering a smooth experience for Chat models. Support for Text-to-Speech (TTS) models will be added in the future when more powerful open-source options become available.
  * 🚄 **Efficient gRPC Communication:** Tokilake and Tokiame communicate via gRPC, minimizing network overhead and latency compared to JSON/HTTP.
  * 🧩 **Flexible Deployment:** Host Tokilake on a simple VPS or a powerful server, while Tokiame clients run on your diverse GPU machines.
  * 🔒 **Dynamic & Secure GPU Registration:** Tokiame clients register with custom `namespaces` (provider names) and an optional `api_secret`. The Tokilake network verifies these credentials by comparing a hash of the provided secret against the securely stored hash in the database, ensuring only authorized access to your valuable GPU resources.

## Why Tokilake?

Compared to other solutions, Tokilake offers a unique blend of benefits for creating a *private, distributed AI inference fabric*:

  * **True Private Cloud Experience:** Unlike public GPU clouds, you use *your own* hardware. Unlike simple remote access tools (VPNs, SSH), Tokilake provides a managed, unified API endpoint.
  * **Unlocks "Hidden" GPUs:** The "no public IP required" feature is a game-changer for leveraging hardware that would otherwise be inaccessible.
  * **Scalability & Resilience by Design:** The distributed Tokilake server architecture with a PostgreSQL backend is built for growth and fault tolerance, mimicking enterprise-grade systems.
  * **Control & Ownership:** You control the infrastructure, the data (if any passes through the proxy), and the access policies.
  * **Simpler than Full Cluster Orchestrators:** While powerful, tools like Kubernetes with GPU support can be overkill for many private setups. Tokilake offers a more streamlined solution for LLM inference access.

## How It Works (Conceptual Overview)

1.  **Setup Database:** A PostgreSQL cluster is set up to store Tokilake's shared state (Tokiame registrations, namespaces, API keys).
2.  **Deploy Tokilake Server(s):** One or more Tokilake server instances are deployed in desired locations. They all connect to the same PostgreSQL database.
3.  **Deploy Tokiame Client(s):** On each machine with GPUs you want to make available:
      * Install your desired inference server (e.g., vLLM, SGLang, llama.cpp).
      * Install and configure Tokiame, pointing it to one of your Tokilake server addresses. Provide a unique `namespace`.
      * Tokiame establishes an outbound gRPC connection to a Tokilake server and registers itself and its models in the shared PostgreSQL database.
4.  **User Access:**
      * Users (or applications) make API requests (e.g., OpenAI-compatible chat completion requests) to any of your Tokilake server URLs.
      * Tokilake authenticates the request, looks up available providers for the requested model in the database, and securely forwards the task via gRPC to an appropriate, connected Tokiame client.
      * Tokiame passes the task to the local inference server, gets the result, and sends it back through Tokilake to the user.

<!-- end list -->

```mermaid
graph LR
    User[End User/Application] -->|API Request| TL_Node1[Tokilake Node 1 e.g. US-East]
    User -->|API Request| TL_Node2[Tokilake Node 2 e.g. EU-West]

    TL_Node1 <-->|Shared State| DB[(PostgreSQL Cluster)]
    TL_Node2 <-->|Shared State| DB

    DB <-->|Registration Info| TM1_Client[Tokiame Client 1 Private GPU 1 - No Public IP]
    DB <-->|Registration Info| TM2_Client[Tokiame Client 2 Private GPU 2 - No Public IP]

    TL_Node1 -->|gRPC Task| TM1_Client
    TL_Node2 -->|gRPC Task| TM2_Client

    TM1_Client -->|Local Inference| GPU1_Server[Inference Server @ GPU1]
    TM2_Client -->|Local Inference| GPU2_Server[Inference Server @ GPU2]

    subgraph Your_Private_Network_1
        TM1_Client
        GPU1_Server
    end

    subgraph Your_Private_Network_2
        TM2_Client
        GPU2_Server
    end
```

## Use Cases

  * **Individuals with Multiple GPU Machines:** Access your home desktop GPU from your laptop while traveling, or your office workstation from home.
  * **Friends & Collaborators:** Pool GPU resources within a trusted group for shared projects without complex networking.
  * **Small Companies/Startups:** Build a cost-effective internal AI inference platform using existing hardware before investing in expensive cloud GPUs.
  * **Researchers:** Easily share access to specialized fine-tuned models hosted on different lab machines.
  * **Edge AI Deployments:** While not its primary focus, the architecture could be adapted for managing inference on distributed edge devices that have intermittent connectivity.

## Getting Started

*Documentation coming soon\!* Details on setting API secrets for Tokiame clients and user authentication will be included.

High-level steps will involve:

1.  Setting up your PostgreSQL database.
2.  Building and configuring the Tokilake server.
3.  Building and configuring the Tokiame client on your GPU machines (including API secret setup).
4.  Pointing your applications to your Tokilake API endpoint.

## Current Status & Roadmap

  * ✅ Core functionality for proxying LLM chat completions.
  * ✅ Tokilake server and Tokiame client implementation with gRPC.
  * ✅ PostgreSQL backend for distributed Tokilake nodes and persistent configuration.
  * 🔜 Support for Text-to-Speech (TTS) models (pending availability of more powerful open-source models).
  * 🔜 Detailed documentation and examples (including API secret configuration and user authentication to Tokilake).
  * 🔜 Enhanced model management and discovery features.
  * 🔜 Support for more inference server backends.
  * 🔜 Observability (logging, metrics).

We are actively developing Tokilake. Contributions and feedback are welcome\!

## Comparison to Alternatives

  * **Cloud LLM APIs (OpenAI, Gemini, etc.):** Offer ease of use but can be expensive and raise privacy concerns for sensitive data. Tokilake lets you use your own models on your own hardware.
  * **API Aggregators (OpenRouter, LiteLLM):** Great for accessing *public* models. Tokilake is your *private* OpenRouter for your *own, non-public* GPUs. LiteLLM can be self-hosted and is excellent for unifying access to various APIs, but Tokilake adds the critical layer of making private, NATted GPUs accessible.
  * **DePIN Compute Projects:** Aim to create public, tokenized markets for GPU power. Tokilake is focused on private, trusted networks without the crypto layer.
  * **VPNs/SSH Tunnels/ngrok/Cloudflare Tunnel:** Useful for general remote access but lack the specialized LLM API gateway features, dynamic registration, load balancing (across your private fleet), and centralized management that Tokilake provides for inference tasks.

## Contributing

We welcome contributions\! Please see `CONTRIBUTING.md` for details on how to get started with development, report issues, and make pull requests.

## License

Tokilake is licensed under the [MIT License](https://www.google.com/search?q=LICENSE).


## Getting Started

This guide will walk you through setting up Tokilake and Tokiame.

**Prerequisites:**

* **Rust and Cargo:** For building the Tokilake server. (Visit [rust-lang.org](https://www.rust-lang.org/tools/install))
* **Make and C++ Compiler:** For building the Tokiame client from source (e.g., `build-essential` on Debian/Ubuntu).
* **Git:** For cloning the repository.
* **PostgreSQL Server:** Running and accessible.
* **psql (PostgreSQL client CLI):** For importing the initial schema (usually part of PostgreSQL server or client packages).
* **An LLM Inference Server:** Such as Ollama, running on your GPU machine(s).

**1. Set Up Your LLM Inference Server (e.g., Ollama)**

Ensure you have an LLM inference server running on the machine(s) where you intend to deploy Tokiame. Tokilake is designed to work with various backends.

* **Ollama:** Install Ollama (see [ollama.com](https://ollama.com/)) and pull the models you want to serve (e.g., `ollama pull llama3`).
    *(Currently, testing has been primarily focused on Ollama. In theory, other backends like SGLang or vLLM, once configured to serve models, should also work with Tokiame.)*

**2. Set Up PostgreSQL Database**

Install PostgreSQL if you haven't already. Then, create a database for Tokilake and import the schema.

```bash
# Example commands (may vary based on your PostgreSQL setup and user privileges)
createdb tokilake_db
# Ensure the path to your SQL migration file is correct. Example:
psql -U your_postgres_user -d tokilake_db -f migrations/001_initial_schema.sql
```
* Replace `your_postgres_user` with your PostgreSQL username. You will be prompted for the password.
* Adjust `migrations/001_initial_schema.sql` if your initial schema file has a different name or path.

**3. Build Tokilake Server and Tokiame Client**

Clone the repository first if you haven't:
```bash
git clone https://github.com/yourusername/tokilake.git # Replace with your actual repository URL
cd tokilake
```

**Build Tokilake Server:**
The Tokilake server is written in Rust.
```bash
cargo build --release
# The binary will be located at ./target/release/tokilake
```

**Build Tokiame Client:**
The Tokiame client is typically built using Make.
```bash
cd tokiame
make
# The binary will usually be located at ./build/bin/tokiame (check its Makefile if different)
cd .. # Return to the project root
```
* *Note: Pre-compiled binaries or Docker images for Tokilake and Tokiame may be provided in the future. For now, please build from source.*
* The Tokilake server consumes very few resources, so you might consider deploying it on a cost-effective VPS or even a free-tier cloud container service.

**4. Run Tokilake Server**

Deploy the Tokilake server binary (`target/release/tokilake`) to your chosen server (e.g., a cloud VPS). It needs the `DATABASE_URL` environment variable to connect to your PostgreSQL database.

```bash
export DATABASE_URL="postgres://your_user:your_password@your_postgres_host:port/tokilake_db"
./target/release/tokilake
```
* Replace `your_user`, `your_password`, `your_postgres_host`, `port`, and `tokilake_db` with your actual PostgreSQL connection details.
* The Tokilake server will listen for incoming gRPC connections from Tokiame clients and HTTP API requests from end-users. Note the IP/hostname and port it's listening on. Check its startup logs or default configuration for specific ports (e.g., HTTP(S) on `0.0.0.0:19981`, gRPC on `0.0.0.0:19982`).

**5. Run Tokiame Client**

Deploy the Tokiame client binary (e.g., `tokiame/build/bin/tokiame`) to each GPU machine that is running your LLM inference server (e.g., Ollama).

When running Tokiame, you need to provide:
* A unique `namespace` for this provider/GPU machine (e.g., "my-rtx4090-desktop").
* The gRPC address of your Tokilake server.
<!-- * An `api_secret` that this Tokiame client will use to register with the Tokilake network. This secret is defined by you; Tokilake will store its hash. -->
* Connection details for your local LLM inference server.

```bash
# Example for a Tokiame client connecting to Ollama running locally
./tokiame/build/bin/tokiame \
  --namespace "my-rtx4090-desktop" \
  --addr "your_tokilake_server_ip_or_domain:19982"
```
* Replace placeholders like `your_tokilake_server_ip_or_domain:19982` with the actual gRPC address of your Tokilake server.
* The models supported by tokiame can be configured in `model.toml`

**6. Enjoy Your Private AI Cloud!**

You can now send OpenAI-compatible API requests to your Tokilake server's HTTP endpoint.

* **Base URL:** Use your Tokilake server's HTTP address (e.g., `http://your_tokilake_server_ip_or_domain:19981`) as the `base_url` in your OpenAI client. Append `/v1` if your server routes OpenAI-compatible requests through it.
* **Model Name:** Specify the model as `namespace:model_name_on_inference_server` (e.g., `my-rtx4090-desktop:llama3` if `llama3` is the model name in Ollama).
* **API Key:** For requests to Tokilake, use the API key mechanism if/when implemented for end-user authentication. For now, you might use a placeholder or a pre-configured static key if required by Tokilake.

Example using the Python `openai` library:
```python
from openai import OpenAI

client = OpenAI(
    base_url="http://your_tokilake_server_ip_or_domain:8000/v1", # Adjust if /v1 is not needed or path is different
    api_key="YOUR_TOKILAKE_USER_API_KEY_IF_APPLICABLE"  # Use a placeholder like "NA" or a real key if Tokilake requires user auth
)

try:
    chat_completion = client.chat.completions.create(
        messages=[
            {
                "role": "user",
                "content": "Explain the Tokilake project in simple terms.",
            }
        ],
        model="my-rtx4090-desktop:llama3" # Use the namespace and model name
    )
    print(chat_completion.choices[0].message.content)
except Exception as e:
    print(f"An error occurred: {e}")

```


## Other ways to play with Tokilake

Beyond creating a private cloud for your own GPUs, Tokilake offers additional functionalities:

**As an API Forwarder for LLMs (OpenRouter-like)**
    * Tokilake can function as an API forwarder for Large Language Models.
    * The endpoint for this feature is `/v2/chat/completion`.
    * This allows you to route your API requests through a unified Tokilake interface, similar to services like OpenRouter.
    * This part refers to the implementation of `aichat` ([https://github.com/sigoden/aichat](https://github.com/sigoden/aichat)) (including code).


## Contributing & Support

This project is in its early stages. If you like it, you are welcome to build it together! Please see `CONTRIBUTING.md` for details on how to get started with development, report issues, and make pull requests.

If you want to support the author, you can sponsor her through cryptocurrency to help her make better progress:


* **Bitcoin:** `bc1pgnls57637tz5ytcf2fmw3f04c35v5dnalg3wycx9qhqvfjdg0ccs62pj63`
* **Ethereum (ETH & ERC-20 Tokens):**  `0x01c264522691cf9142b35aa808a9d518e4bc7028`
* **Solana (SOL & SPL Tokens):** `2KLf23ypzYKfNR7EsSCa8EikUAHdtF1Jr18dgA2WaHxH`