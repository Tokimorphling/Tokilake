package api

import (
	"net/http"
	"tokiame/pkg/config"
	"tokiame/pkg/log"

	"github.com/gin-gonic/gin"
)

type AddModelRequest struct {
	Model config.ModelDetails `json:"model" binding:"required"`
}

// DeleteModelRequest represents the request body for deleting a model.
type DeleteModelRequest struct {
	ID string `json:"id" binding:"required"`
}

// RegisterConfigAPI registers the API endpoints related to model configuration.
func RegisterModelConfigAPI(router *gin.Engine, cfgManager *config.Manager) {
	configGroup := router.Group("/api")
	{
		configGroup.POST("/models", addModelHandler(cfgManager))
		configGroup.DELETE("/models", deleteModelHandler(cfgManager))
		configGroup.GET("/models", getModelsHandler(cfgManager))
	}
}

func getModelsHandler(cfgManager *config.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentConfig := cfgManager.Get()
		if currentConfig == nil {
			log.Errorf("Attempted to retrieve models, but current configuration is nil.")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Configuration not loaded"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"models": currentConfig.SupportedModels})
	}
}

// addModelHandler handles the POST request to add a new model.
func addModelHandler(cfgManager *config.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req AddModelRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Errorf("Add model request binding error: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Acquire a lock to safely modify the configuration

		cfgManager.Mu.Lock()

		// Check if the model ID already exists
		for _, existingModel := range cfgManager.Current.SupportedModels {
			if existingModel.Id == req.Model.Id {
				log.Warnf("Attempted to add existing model ID: %s", req.Model.Id)
				c.JSON(http.StatusConflict, gin.H{"error": "Model with this ID already exists"})
				return
			}
		}

		// Add the new model
		cfgManager.Current.SupportedModels = append(cfgManager.Current.SupportedModels, &req.Model)
		cfgManager.Mu.Unlock()

		// Save the updated configuration to the file
		if err := cfgManager.Save(); err != nil {
			log.Errorf("Failed to save configuration after adding model %s: %v", req.Model.Id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
			return
		}

		log.Infof("Successfully added model: %s", req.Model.Id)
		c.JSON(http.StatusCreated, gin.H{"message": "Model added successfully", "model_id": req.Model.Id})
	}
}

// deleteModelHandler handles the DELETE request to remove an existing model.
func deleteModelHandler(cfgManager *config.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req DeleteModelRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Errorf("Delete model request binding error: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Acquire a lock to safely modify the configuration
		cfgManager.Mu.Lock()

		found := false
		updatedModels := []*config.ModelDetails{}
		for _, model := range cfgManager.Current.SupportedModels {
			if model.Id == req.ID {
				found = true
				log.Infof("Deleting model: %s", req.ID)
			} else {
				updatedModels = append(updatedModels, model)
			}
		}

		if !found {
			log.Warnf("Attempted to delete non-existent model ID: %s", req.ID)
			c.JSON(http.StatusNotFound, gin.H{"error": "Model with this ID not found"})
			return
		}

		cfgManager.Current.SupportedModels = updatedModels
		cfgManager.Mu.Unlock()

		// Save the updated configuration to the file
		if err := cfgManager.Save(); err != nil {
			log.Errorf("Failed to save configuration after deleting model %s: %v", req.ID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
			return
		}

		log.Infof("Successfully deleted model: %s", req.ID)
		c.JSON(http.StatusOK, gin.H{"message": "Model deleted successfully", "model_id": req.ID})
	}
}
