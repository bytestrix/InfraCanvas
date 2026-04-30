package kubernetes

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"infracanvas/internal/models"
)

// GetDeployments collects all deployments in the specified namespace (empty string for all namespaces)
func (d *Discovery) GetDeployments(ctx context.Context, namespace string) ([]models.Deployment, error) {
	cacheKey := fmt.Sprintf("deployments:%s", namespace)
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if deployments, ok := cached.([]models.Deployment); ok {
			return deployments, nil
		}
	}
	
	deployList, err := d.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	deployments := make([]models.Deployment, 0, len(deployList.Items))
	for _, deploy := range deployList.Items {
		deployment := d.parseDeployment(&deploy)
		deployments = append(deployments, deployment)
	}

	// Cache the result
	d.cache.Set(cacheKey, deployments)

	return deployments, nil
}

func (d *Discovery) parseDeployment(deploy *appsv1.Deployment) models.Deployment {
	replicas := int32(0)
	if deploy.Spec.Replicas != nil {
		replicas = *deploy.Spec.Replicas
	}

	containers := parseContainerSpecs(deploy.Spec.Template.Spec.Containers)

	// Determine health
	health := models.HealthHealthy
	if deploy.Status.AvailableReplicas < replicas {
		if deploy.Status.AvailableReplicas == 0 {
			health = models.HealthUnhealthy
		} else {
			health = models.HealthDegraded
		}
	}

	strategy := "RollingUpdate"
	if deploy.Spec.Strategy.Type != "" {
		strategy = string(deploy.Spec.Strategy.Type)
	}

	pullSecrets := make([]string, 0, len(deploy.Spec.Template.Spec.ImagePullSecrets))
	for _, s := range deploy.Spec.Template.Spec.ImagePullSecrets {
		pullSecrets = append(pullSecrets, s.Name)
	}

	deployment := models.Deployment{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("deployment/%s/%s", deploy.Namespace, deploy.Name),
			Type:        models.EntityTypeDeployment,
			Labels:      deploy.Labels,
			Annotations: deploy.Annotations,
			Health:      health,
			Timestamp:   time.Now(),
		},
		Name:               deploy.Name,
		Namespace:          deploy.Namespace,
		Replicas:           replicas,
		AvailableReplicas:  deploy.Status.AvailableReplicas,
		ReadyReplicas:      deploy.Status.ReadyReplicas,
		UpdatedReplicas:    deploy.Status.UpdatedReplicas,
		Selector:           deploy.Spec.Selector.MatchLabels,
		Containers:         containers,
		Strategy:           strategy,
		Generation:         deploy.Generation,
		ObservedGeneration: deploy.Status.ObservedGeneration,
		ServiceAccount:     deploy.Spec.Template.Spec.ServiceAccountName,
		ImagePullSecrets:   pullSecrets,
		ChartVersion:       deploy.Labels["helm.sh/chart"],
		HelmRelease:        deploy.Annotations["meta.helm.sh/release-name"],
	}

	return deployment
}

// parseContainerSpecs extracts ContainerSpec list from k8s containers including
// ports and env-key (not value) info so the UI can show shape without leaking secrets.
func parseContainerSpecs(cs []corev1.Container) []models.ContainerSpec {
	out := make([]models.ContainerSpec, 0, len(cs))
	for _, c := range cs {
		ports := make([]models.ContainerPort, 0, len(c.Ports))
		for _, p := range c.Ports {
			ports = append(ports, models.ContainerPort{
				Name:          p.Name,
				ContainerPort: p.ContainerPort,
				Protocol:      string(p.Protocol),
			})
		}
		envKeys := make([]string, 0, len(c.Env))
		for _, e := range c.Env {
			envKeys = append(envKeys, e.Name)
		}
		envFrom := make([]string, 0, len(c.EnvFrom))
		for _, ef := range c.EnvFrom {
			if ef.ConfigMapRef != nil {
				envFrom = append(envFrom, "configmap/"+ef.ConfigMapRef.Name)
			}
			if ef.SecretRef != nil {
				envFrom = append(envFrom, "secret/"+ef.SecretRef.Name)
			}
		}
		out = append(out, models.ContainerSpec{
			Name:  c.Name,
			Image: c.Image,
			Resources: models.ResourceRequirements{
				Requests: parseResourceList(c.Resources.Requests),
				Limits:   parseResourceList(c.Resources.Limits),
			},
			Ports:   ports,
			EnvKeys: envKeys,
			EnvFrom: envFrom,
		})
	}
	return out
}

// GetStatefulSets collects all statefulsets in the specified namespace
func (d *Discovery) GetStatefulSets(ctx context.Context, namespace string) ([]models.StatefulSet, error) {
	cacheKey := fmt.Sprintf("statefulsets:%s", namespace)
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if statefulsets, ok := cached.([]models.StatefulSet); ok {
			return statefulsets, nil
		}
	}
	
	stsList, err := d.clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list statefulsets: %w", err)
	}

	statefulsets := make([]models.StatefulSet, 0, len(stsList.Items))
	for _, sts := range stsList.Items {
		statefulset := d.parseStatefulSet(&sts)
		statefulsets = append(statefulsets, statefulset)
	}

	// Cache the result
	d.cache.Set(cacheKey, statefulsets)

	return statefulsets, nil
}

func (d *Discovery) parseStatefulSet(sts *appsv1.StatefulSet) models.StatefulSet {
	replicas := int32(0)
	if sts.Spec.Replicas != nil {
		replicas = *sts.Spec.Replicas
	}

	containers := parseContainerSpecs(sts.Spec.Template.Spec.Containers)

	// Parse volume claim templates
	vcts := make([]string, 0, len(sts.Spec.VolumeClaimTemplates))
	for _, vct := range sts.Spec.VolumeClaimTemplates {
		vcts = append(vcts, vct.Name)
	}

	// Determine health
	health := models.HealthHealthy
	if sts.Status.ReadyReplicas < replicas {
		if sts.Status.ReadyReplicas == 0 {
			health = models.HealthUnhealthy
		} else {
			health = models.HealthDegraded
		}
	}

	statefulset := models.StatefulSet{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("statefulset/%s/%s", sts.Namespace, sts.Name),
			Type:        models.EntityTypeStatefulSet,
			Labels:      sts.Labels,
			Annotations: sts.Annotations,
			Health:      health,
			Timestamp:   time.Now(),
		},
		Name:                 sts.Name,
		Namespace:            sts.Namespace,
		Replicas:             replicas,
		ReadyReplicas:        sts.Status.ReadyReplicas,
		CurrentReplicas:      sts.Status.CurrentReplicas,
		UpdatedReplicas:      sts.Status.UpdatedReplicas,
		ServiceName:          sts.Spec.ServiceName,
		Selector:             sts.Spec.Selector.MatchLabels,
		Containers:           containers,
		VolumeClaimTemplates: vcts,
	}

	return statefulset
}

// GetDaemonSets collects all daemonsets in the specified namespace
func (d *Discovery) GetDaemonSets(ctx context.Context, namespace string) ([]models.DaemonSet, error) {
	cacheKey := fmt.Sprintf("daemonsets:%s", namespace)
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if daemonsets, ok := cached.([]models.DaemonSet); ok {
			return daemonsets, nil
		}
	}
	
	dsList, err := d.clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list daemonsets: %w", err)
	}

	daemonsets := make([]models.DaemonSet, 0, len(dsList.Items))
	for _, ds := range dsList.Items {
		daemonset := d.parseDaemonSet(&ds)
		daemonsets = append(daemonsets, daemonset)
	}

	// Cache the result
	d.cache.Set(cacheKey, daemonsets)

	return daemonsets, nil
}

func (d *Discovery) parseDaemonSet(ds *appsv1.DaemonSet) models.DaemonSet {
	containers := parseContainerSpecs(ds.Spec.Template.Spec.Containers)

	// Determine health
	health := models.HealthHealthy
	if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
		if ds.Status.NumberReady == 0 {
			health = models.HealthUnhealthy
		} else {
			health = models.HealthDegraded
		}
	}

	daemonset := models.DaemonSet{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("daemonset/%s/%s", ds.Namespace, ds.Name),
			Type:        models.EntityTypeDaemonSet,
			Labels:      ds.Labels,
			Annotations: ds.Annotations,
			Health:      health,
			Timestamp:   time.Now(),
		},
		Name:                   ds.Name,
		Namespace:              ds.Namespace,
		DesiredNumberScheduled: ds.Status.DesiredNumberScheduled,
		CurrentNumberScheduled: ds.Status.CurrentNumberScheduled,
		NumberReady:            ds.Status.NumberReady,
		NumberAvailable:        ds.Status.NumberAvailable,
		Selector:               ds.Spec.Selector.MatchLabels,
		Containers:             containers,
	}

	return daemonset
}

// GetJobs collects all jobs in the specified namespace
func (d *Discovery) GetJobs(ctx context.Context, namespace string) ([]models.Job, error) {
	cacheKey := fmt.Sprintf("jobs:%s", namespace)
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if jobs, ok := cached.([]models.Job); ok {
			return jobs, nil
		}
	}
	
	jobList, err := d.clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	jobs := make([]models.Job, 0, len(jobList.Items))
	for _, j := range jobList.Items {
		job := d.parseJob(&j)
		jobs = append(jobs, job)
	}

	// Cache the result
	d.cache.Set(cacheKey, jobs)

	return jobs, nil
}

func (d *Discovery) parseJob(j *batchv1.Job) models.Job {
	completions := int32(1)
	if j.Spec.Completions != nil {
		completions = *j.Spec.Completions
	}

	parallelism := int32(1)
	if j.Spec.Parallelism != nil {
		parallelism = *j.Spec.Parallelism
	}

	ownerKind := ""
	ownerName := ""
	if len(j.OwnerReferences) > 0 {
		ownerKind = j.OwnerReferences[0].Kind
		ownerName = j.OwnerReferences[0].Name
	}

	// Determine health
	health := models.HealthHealthy
	if j.Status.Failed > 0 {
		health = models.HealthUnhealthy
	} else if j.Status.Active > 0 {
		health = models.HealthDegraded
	}

	startTime := time.Time{}
	if j.Status.StartTime != nil {
		startTime = j.Status.StartTime.Time
	}

	completionTime := time.Time{}
	if j.Status.CompletionTime != nil {
		completionTime = j.Status.CompletionTime.Time
	}

	job := models.Job{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("job/%s/%s", j.Namespace, j.Name),
			Type:        models.EntityTypeJob,
			Labels:      j.Labels,
			Annotations: j.Annotations,
			Health:      health,
			Timestamp:   time.Now(),
		},
		Name:           j.Name,
		Namespace:      j.Namespace,
		Completions:    completions,
		Parallelism:    parallelism,
		Active:         j.Status.Active,
		Succeeded:      j.Status.Succeeded,
		Failed:         j.Status.Failed,
		StartTime:      startTime,
		CompletionTime: completionTime,
		OwnerKind:      ownerKind,
		OwnerName:      ownerName,
	}

	return job
}

// GetCronJobs collects all cronjobs in the specified namespace
func (d *Discovery) GetCronJobs(ctx context.Context, namespace string) ([]models.CronJob, error) {
	cacheKey := fmt.Sprintf("cronjobs:%s", namespace)
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if cronjobs, ok := cached.([]models.CronJob); ok {
			return cronjobs, nil
		}
	}
	
	cronList, err := d.clientset.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list cronjobs: %w", err)
	}

	cronjobs := make([]models.CronJob, 0, len(cronList.Items))
	for _, cj := range cronList.Items {
		cronjob := d.parseCronJob(&cj)
		cronjobs = append(cronjobs, cronjob)
	}

	// Cache the result
	d.cache.Set(cacheKey, cronjobs)

	return cronjobs, nil
}

func (d *Discovery) parseCronJob(cj *batchv1.CronJob) models.CronJob {
	suspend := false
	if cj.Spec.Suspend != nil {
		suspend = *cj.Spec.Suspend
	}

	lastScheduleTime := time.Time{}
	if cj.Status.LastScheduleTime != nil {
		lastScheduleTime = cj.Status.LastScheduleTime.Time
	}

	// Determine health
	health := models.HealthHealthy
	if suspend {
		health = models.HealthDegraded
	}

	cronjob := models.CronJob{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("cronjob/%s/%s", cj.Namespace, cj.Name),
			Type:        models.EntityTypeCronJob,
			Labels:      cj.Labels,
			Annotations: cj.Annotations,
			Health:      health,
			Timestamp:   time.Now(),
		},
		Name:             cj.Name,
		Namespace:        cj.Namespace,
		Schedule:         cj.Spec.Schedule,
		Suspend:          suspend,
		LastScheduleTime: lastScheduleTime,
		Active:           len(cj.Status.Active),
	}

	return cronjob
}
