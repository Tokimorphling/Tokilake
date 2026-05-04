//! Relay module — OpenAI-compatible API passthrough.
//!
//! This module will house the logic for forwarding `/v1/chat/completions`
//! and other OpenAI-compatible endpoints through the gateway service stack.
//!
//! Auth is now handled by `gateway::auth::AuthService` as a composable layer.
