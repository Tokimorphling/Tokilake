mod chat_completion;
mod private_chat_completions;
mod public_models;

pub use chat_completion::chat_completion_router;
pub use private_chat_completions::private_chat_completion_router;
pub use public_models::public_models_router;
