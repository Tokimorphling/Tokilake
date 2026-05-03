# Tokilake vs LiteLLM vs one-api vs new-api vs exo

When building an AI application or managing an LLM infrastructure, choosing the right API gateway or orchestration tool is critical. 

While tools like **LiteLLM**, **one-api**, and **exo** are excellent at what they do, **Tokilake** is built to solve a very specific infrastructure bottleneck: **Monetizing and pooling distributed, private GPUs (like local RTX 4090s) behind NAT without exposing public IPs.**

This document provides a detailed technical and architectural comparison to help you choose the right tool for your use case.

---

## 1. The Core Architectural Difference: Static Proxy vs. Reverse Tunnel

Almost all traditional LLM gateways (LiteLLM, one-api, Kong) operate on a **Static Proxy** model. They assume that your backend model (whether it's OpenAI, Anthropic, or a self-hosted vLLM instance) has a **publicly reachable IP address or a static internal URL**. 

If you want to add your home computer running Ollama to LiteLLM, you must set up port forwarding on your router, configure a reverse proxy like Nginx, or pay for Ngrok. This is a massive security risk and operational headache.

**Tokilake uses a Reverse Tunnel (Worker-Initiated) model.**
Using the `tokiame` client, your home GPU actively "dials out" to the Tokilake central hub using a secure WebSocket or QUIC tunnel. 
- **Zero Inbound Ports**: Your router requires zero configuration.
- **NAT Traversal**: Works out of the box from home connections, campus networks, or highly restricted enterprise intranets.
- **Auto-Failover**: If your home PC loses power, Tokilake instantly drops the node from the load-balancing pool.

---

## 2. Tokilake vs LiteLLM

[LiteLLM](https://github.com/BerriAI/litellm) is the most popular Python-based LLM gateway. It is incredible for standardizing 100+ API providers into the OpenAI format and is deeply integrated into the Python/LangChain ecosystem.

*   **Language**: LiteLLM (Python) vs. Tokilake (Go). Tokilake benefits from Go's raw high-concurrency performance and compiled binary deployment.
*   **Worker Registration**: LiteLLM requires you to manually add endpoints to a `config.yaml` or database. Tokilake allows workers (`tokiame`) to dynamically self-register via invite codes.
*   **The Verdict**: Use **LiteLLM** if your entire stack is Python, you only use public Cloud APIs (OpenAI, AWS Bedrock), and you want rapid integration. Use **Tokilake** if you are aggregating your own private, scattered GPU hardware into a unified API.

---

## 3. Tokilake vs one-api / new-api

[one-api](https://github.com/songquanpeng/one-api) and its popular fork [new-api](https://github.com/Calcium-Ion/new-api) are the foundational inspirations for Tokilake. 

*   **The Relationship**: Tokilake is actually a hard-fork and architectural evolution of the one-api ecosystem. Tokilake retains the excellent billing system, multi-tenant grouping, and dashboard UI of one-api.
*   **The Difference**: one-api is purely an API relayer. You paste a Base URL and an API Key, and it forwards the request. **Tokilake adds the `tokilake-core` networking engine**. It introduces the concept of a `Worker` node.
*   **The Verdict**: If you are just reselling public API keys, `new-api` is perfectly fine. If you want to build a decentralized OpenRouter clone using community-provided GPUs, **Tokilake** is the only one equipped for the job.

---

## 4. Tokilake vs exo

[exo](https://github.com/exo-explore/exo) is a brilliant project that allows you to run your own AI cluster at home.

*   **Different Goals**: **exo** is designed for **Model Sharding**. If you have three MacBooks and want to run a massive 70B parameter model that doesn't fit in one machine's RAM, exo splits the model across the three laptops.
*   **Tokilake's Goal**: Tokilake does not shard models. It is designed for **Request Routing**. If you have three laptops, Tokilake assumes laptop A is running Llama-3 (8B), laptop B is running Qwen-Image, and laptop C is running ComfyUI. Tokilake routes user API requests to the appropriate machine.
*   **Network Limits**: exo requires high-bandwidth, low-latency LAN connections (like Thunderbolt or gigabit Ethernet) because it passes tensors between machines. Tokilake works perfectly over the global public internet, connecting a node in Tokyo with a hub in New York.

---

## 5. Comprehensive Feature Matrix

| Capability | Tokilake | LiteLLM | one-api / new-api | exo |
| :--- | :---: | :---: | :---: | :---: |
| **OpenAI-Compatible API** | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes |
| **Multi-Provider Aggregation** | ✅ Yes | ✅ Yes | ✅ Yes | ❌ No |
| **Billing & Quota Management** | ✅ Yes | ✅ Yes (Enterprise) | ✅ Yes | ❌ No |
| **Admin Dashboard UI** | ✅ Built-in | ✅ Built-in | ✅ Built-in | ❌ CLI only |
| **Tunnel-based NAT Traversal** | ✅ **Yes (QUIC/WS)** | ❌ No | ❌ No | ❌ No |
| **Remote Worker Self-Registration**| ✅ **Yes** | ❌ No | ❌ No | ✅ Yes (P2P LAN) |
| **Zero Inbound Ports on Worker** | ✅ **Yes** | ❌ No | ❌ No | ❌ No |
| **Model Sharding (Tensor Parallelism)**| ❌ No | ❌ No | ❌ No | ✅ **Yes** |
| **Heterogeneous Hardware (Cloud+Home)**| ✅ Yes | ❌ N/A | ❌ N/A | ⚠️ Limited |

## Summary: When to choose Tokilake?

Choose **Tokilake** if:
1. You want to expose your local Ollama, vLLM, or ComfyUI instances to the internet without setting up reverse proxies or paying for Ngrok.
2. You have GPUs scattered across different physical locations or cloud providers and want to pool them behind a single, monetizable API Endpoint.
3. You are building a decentralized compute sharing platform where users can securely connect their hardware to your network.
