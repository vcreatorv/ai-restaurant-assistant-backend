// Package usecase реализует chat.Usecase.
package usecase

import (
	"github.com/example/ai-restaurant-assistant-backend/internal/chat"
	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/cohere"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/llm"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/qdrant"
	"github.com/example/ai-restaurant-assistant-backend/internal/prompts"
	"github.com/example/ai-restaurant-assistant-backend/internal/rag"
	"github.com/example/ai-restaurant-assistant-backend/internal/user"
)

// Deps зависимости chat.Usecase
type Deps struct {
	// Repo репозиторий чатов и сообщений
	Repo chat.Repository
	// UUID генератор UUID
	UUID chat.UUIDGen
	// Users user-фасад (профиль гостя — allergens/dietary)
	Users user.Usecase
	// Menu menu-фасад (batch-load блюд по id)
	Menu menu.Usecase
	// Cohere клиент Cohere (Embed + Rerank)
	Cohere *cohere.Client
	// Qdrant клиент Qdrant (Search)
	Qdrant *qdrant.Client
	// LLM клиент LLM-провайдера (OpenRouter или NVIDIA NIM); собирается фабрикой
	// internal/pkg/openrouter или internal/pkg/nvidia в зависимости от rag.llm.provider
	LLM *llm.Client
	// Classifier клиент LLM для классификатора намерений (recommend/clarify/chitchat/off_topic).
	// Может быть тем же *llm.Client, что и LLM (если в конфиге не задан отдельный classifier),
	// либо более лёгкой моделью (cheaper/faster).
	Classifier *llm.Client
	// Prompts источник системного промпта (с поддержкой личных draft-ов админов)
	Prompts prompts.Usecase
	// ChatCfg параметры chat-фичи (auto_new_chat_after, ...)
	ChatCfg chat.UsecaseConfig
	// RAGCfg параметры RAG (top_k, rerank_top_n, history_limit, ...)
	RAGCfg rag.Config
}

type chatUsecase struct {
	repo       chat.Repository
	uuid       chat.UUIDGen
	users      user.Usecase
	menu       menu.Usecase
	cohere     *cohere.Client
	qdrant     *qdrant.Client
	llm        *llm.Client
	classifier *llm.Client
	prompts    prompts.Usecase
	chatCfg    chat.UsecaseConfig
	ragCfg     rag.Config
}

// New создаёт chat.Usecase
func New(d Deps) chat.Usecase {
	classifier := d.Classifier
	if classifier == nil {
		classifier = d.LLM
	}
	return &chatUsecase{
		repo:       d.Repo,
		uuid:       d.UUID,
		users:      d.Users,
		menu:       d.Menu,
		cohere:     d.Cohere,
		qdrant:     d.Qdrant,
		llm:        d.LLM,
		classifier: classifier,
		prompts:    d.Prompts,
		chatCfg:    d.ChatCfg,
		ragCfg:     d.RAGCfg,
	}
}
