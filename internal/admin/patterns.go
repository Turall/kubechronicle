package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kubechronicle/kubechronicle/internal/config"
)

// PatternsHandler handles admin endpoints for managing ignore and block patterns.
type PatternsHandler struct {
	clientset  kubernetes.Interface
	namespace  string
	configMapName string
}

// NewPatternsHandler creates a new patterns handler.
func NewPatternsHandler(clientset kubernetes.Interface, namespace, configMapName string) *PatternsHandler {
	return &PatternsHandler{
		clientset:     clientset,
		namespace:     namespace,
		configMapName: configMapName,
	}
}

// HandleGetIgnoreConfig handles GET /api/admin/patterns/ignore.
func (h *PatternsHandler) HandleGetIgnoreConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		h.handleOptions(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	configMap, err := h.getConfigMap(r.Context())
	if err != nil {
		klog.Errorf("Failed to get ConfigMap: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get configuration: %v", err), http.StatusInternalServerError)
		return
	}

	// Extract ignore config from ConfigMap
	ignoreJSON := configMap.Data["IGNORE_CONFIG"]
	if ignoreJSON == "" {
		// Return empty config if not set
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&config.IgnoreConfig{})
		return
	}

	var ignoreConfig config.IgnoreConfig
	if err := json.Unmarshal([]byte(ignoreJSON), &ignoreConfig); err != nil {
		klog.Errorf("Failed to parse ignore config: %v", err)
		http.Error(w, fmt.Sprintf("Failed to parse configuration: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ignoreConfig)
}

// HandleUpdateIgnoreConfig handles PUT /api/admin/patterns/ignore.
func (h *PatternsHandler) HandleUpdateIgnoreConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		h.handleOptions(w, r)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var ignoreConfig config.IgnoreConfig
	if err := json.NewDecoder(r.Body).Decode(&ignoreConfig); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate and update ConfigMap
	if err := h.updateConfigMap(r.Context(), "IGNORE_CONFIG", &ignoreConfig, nil); err != nil {
		klog.Errorf("Failed to update ignore config: %v", err)
		http.Error(w, fmt.Sprintf("Failed to update configuration: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ignoreConfig)
}

// HandleGetBlockConfig handles GET /api/admin/patterns/block.
func (h *PatternsHandler) HandleGetBlockConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		h.handleOptions(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	configMap, err := h.getConfigMap(r.Context())
	if err != nil {
		klog.Errorf("Failed to get ConfigMap: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get configuration: %v", err), http.StatusInternalServerError)
		return
	}

	// Extract block config from ConfigMap
	blockJSON := configMap.Data["BLOCK_CONFIG"]
	if blockJSON == "" {
		// Return empty config if not set
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&config.BlockConfig{})
		return
	}

	var blockConfig config.BlockConfig
	if err := json.Unmarshal([]byte(blockJSON), &blockConfig); err != nil {
		klog.Errorf("Failed to parse block config: %v", err)
		http.Error(w, fmt.Sprintf("Failed to parse configuration: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(blockConfig)
}

// HandleUpdateBlockConfig handles PUT /api/admin/patterns/block.
func (h *PatternsHandler) HandleUpdateBlockConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		h.handleOptions(w, r)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var blockConfig config.BlockConfig
	if err := json.NewDecoder(r.Body).Decode(&blockConfig); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Set default message if not provided
	if blockConfig.Message == "" {
		blockConfig.Message = "Resource blocked by kubechronicle policy"
	}

	// Validate and update ConfigMap
	if err := h.updateConfigMap(r.Context(), "BLOCK_CONFIG", nil, &blockConfig); err != nil {
		klog.Errorf("Failed to update block config: %v", err)
		http.Error(w, fmt.Sprintf("Failed to update configuration: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(blockConfig)
}

// getConfigMap retrieves the ConfigMap.
func (h *PatternsHandler) getConfigMap(ctx context.Context) (*corev1.ConfigMap, error) {
	cm, err := h.clientset.CoreV1().ConfigMaps(h.namespace).Get(ctx, h.configMapName, metav1.GetOptions{})
	if err != nil {
		// If ConfigMap doesn't exist, create it
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      h.configMapName,
				Namespace: h.namespace,
			},
			Data: make(map[string]string),
		}
		cm, err = h.clientset.CoreV1().ConfigMaps(h.namespace).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create ConfigMap: %w", err)
		}
	}
	return cm, nil
}

// updateConfigMap updates the ConfigMap with new pattern configuration.
func (h *PatternsHandler) updateConfigMap(ctx context.Context, key string, ignoreConfig *config.IgnoreConfig, blockConfig *config.BlockConfig) error {
	cm, err := h.getConfigMap(ctx)
	if err != nil {
		return err
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	if ignoreConfig != nil {
		ignoreJSON, err := json.Marshal(ignoreConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal ignore config: %w", err)
		}
		cm.Data["IGNORE_CONFIG"] = string(ignoreJSON)
	}

	if blockConfig != nil {
		blockJSON, err := json.Marshal(blockConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal block config: %w", err)
		}
		cm.Data["BLOCK_CONFIG"] = string(blockJSON)
	}

	_, err = h.clientset.CoreV1().ConfigMaps(h.namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap: %w", err)
	}

	return nil
}

// handleOptions handles CORS preflight requests.
func (h *PatternsHandler) handleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.WriteHeader(http.StatusOK)
}
