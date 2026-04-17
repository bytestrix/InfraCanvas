package kubernetes

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"infracanvas/internal/models"
)

// GetPVCs collects all persistent volume claims in the specified namespace
func (d *Discovery) GetPVCs(ctx context.Context, namespace string) ([]models.PersistentVolumeClaim, error) {
	cacheKey := fmt.Sprintf("pvcs:%s", namespace)
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if pvcs, ok := cached.([]models.PersistentVolumeClaim); ok {
			return pvcs, nil
		}
	}
	
	pvcList, err := d.clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pvcs: %w", err)
	}

	pvcs := make([]models.PersistentVolumeClaim, 0, len(pvcList.Items))
	for _, pvc := range pvcList.Items {
		pvcModel := d.parsePVC(&pvc)
		pvcs = append(pvcs, pvcModel)
	}

	// Cache the result
	d.cache.Set(cacheKey, pvcs)

	return pvcs, nil
}

func (d *Discovery) parsePVC(pvc *corev1.PersistentVolumeClaim) models.PersistentVolumeClaim {
	storageClass := ""
	if pvc.Spec.StorageClassName != nil {
		storageClass = *pvc.Spec.StorageClassName
	}

	requestedStorage := ""
	if storage, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		requestedStorage = storage.String()
	}

	accessModes := make([]string, 0, len(pvc.Spec.AccessModes))
	for _, mode := range pvc.Spec.AccessModes {
		accessModes = append(accessModes, string(mode))
	}

	// Determine health
	health := models.HealthHealthy
	if pvc.Status.Phase != corev1.ClaimBound {
		health = models.HealthDegraded
	}

	pvcModel := models.PersistentVolumeClaim{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("pvc/%s/%s", pvc.Namespace, pvc.Name),
			Type:        models.EntityTypePVC,
			Labels:      pvc.Labels,
			Annotations: pvc.Annotations,
			Health:      health,
			Timestamp:   time.Now(),
		},
		Name:             pvc.Name,
		Namespace:        pvc.Namespace,
		Status:           string(pvc.Status.Phase),
		StorageClass:     storageClass,
		RequestedStorage: requestedStorage,
		AccessModes:      accessModes,
		VolumeName:       pvc.Spec.VolumeName,
	}

	return pvcModel
}

// GetPVs collects all persistent volumes
func (d *Discovery) GetPVs(ctx context.Context) ([]models.PersistentVolume, error) {
	cacheKey := "pvs"
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if pvs, ok := cached.([]models.PersistentVolume); ok {
			return pvs, nil
		}
	}
	
	pvList, err := d.clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pvs: %w", err)
	}

	pvs := make([]models.PersistentVolume, 0, len(pvList.Items))
	for _, pv := range pvList.Items {
		pvModel := d.parsePV(&pv)
		pvs = append(pvs, pvModel)
	}

	// Cache the result
	d.cache.Set(cacheKey, pvs)

	return pvs, nil
}

func (d *Discovery) parsePV(pv *corev1.PersistentVolume) models.PersistentVolume {
	capacity := ""
	if storage, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		capacity = storage.String()
	}

	accessModes := make([]string, 0, len(pv.Spec.AccessModes))
	for _, mode := range pv.Spec.AccessModes {
		accessModes = append(accessModes, string(mode))
	}

	claimRef := ""
	if pv.Spec.ClaimRef != nil {
		claimRef = fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
	}

	// Determine health
	health := models.HealthHealthy
	if pv.Status.Phase != corev1.VolumeBound && pv.Status.Phase != corev1.VolumeAvailable {
		health = models.HealthDegraded
	}

	pvModel := models.PersistentVolume{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("pv/%s", pv.Name),
			Type:        models.EntityTypePV,
			Labels:      pv.Labels,
			Annotations: pv.Annotations,
			Health:      health,
			Timestamp:   time.Now(),
		},
		Name:          pv.Name,
		Capacity:      capacity,
		AccessModes:   accessModes,
		ReclaimPolicy: string(pv.Spec.PersistentVolumeReclaimPolicy),
		Status:        string(pv.Status.Phase),
		StorageClass:  pv.Spec.StorageClassName,
		ClaimRef:      claimRef,
	}

	return pvModel
}

// GetStorageClasses collects all storage classes
func (d *Discovery) GetStorageClasses(ctx context.Context) ([]models.StorageClass, error) {
	cacheKey := "storageclasses"
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if storageclasses, ok := cached.([]models.StorageClass); ok {
			return storageclasses, nil
		}
	}
	
	scList, err := d.clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list storageclasses: %w", err)
	}

	storageclasses := make([]models.StorageClass, 0, len(scList.Items))
	for _, sc := range scList.Items {
		storageclass := d.parseStorageClass(&sc)
		storageclasses = append(storageclasses, storageclass)
	}

	// Cache the result
	d.cache.Set(cacheKey, storageclasses)

	return storageclasses, nil
}

func (d *Discovery) parseStorageClass(sc *storagev1.StorageClass) models.StorageClass {
	reclaimPolicy := "Delete"
	if sc.ReclaimPolicy != nil {
		reclaimPolicy = string(*sc.ReclaimPolicy)
	}

	volumeBindingMode := "Immediate"
	if sc.VolumeBindingMode != nil {
		volumeBindingMode = string(*sc.VolumeBindingMode)
	}

	parameters := make(map[string]string)
	if sc.Parameters != nil {
		parameters = sc.Parameters
	}

	storageclass := models.StorageClass{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("storageclass/%s", sc.Name),
			Type:        models.EntityTypeStorageClass,
			Labels:      sc.Labels,
			Annotations: sc.Annotations,
			Health:      models.HealthHealthy,
			Timestamp:   time.Now(),
		},
		Name:              sc.Name,
		Provisioner:       sc.Provisioner,
		ReclaimPolicy:     reclaimPolicy,
		VolumeBindingMode: volumeBindingMode,
		Parameters:        parameters,
	}

	return storageclass
}
