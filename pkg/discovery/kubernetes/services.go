package kubernetes

import (
	"context"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"infracanvas/internal/models"
)

// GetServices collects all services in the specified namespace
func (d *Discovery) GetServices(ctx context.Context, namespace string) ([]models.K8sService, error) {
	cacheKey := fmt.Sprintf("services:%s", namespace)
	
	// Check cache first
	if cached, found := d.cache.Get(cacheKey); found {
		if services, ok := cached.([]models.K8sService); ok {
			return services, nil
		}
	}
	
	svcList, err := d.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	services := make([]models.K8sService, 0, len(svcList.Items))
	for _, svc := range svcList.Items {
		service := d.parseService(ctx, &svc)
		services = append(services, service)
	}

	// Cache the result
	d.cache.Set(cacheKey, services)

	return services, nil
}

func (d *Discovery) parseService(ctx context.Context, svc *corev1.Service) models.K8sService {
	// Parse ports
	ports := make([]models.ServicePort, 0, len(svc.Spec.Ports))
	for _, p := range svc.Spec.Ports {
		ports = append(ports, models.ServicePort{
			Name:       p.Name,
			Protocol:   string(p.Protocol),
			Port:       p.Port,
			TargetPort: p.TargetPort.String(),
			NodePort:   p.NodePort,
		})
	}

	// Check if service has endpoints
	hasEndpoints := false
	endpoints, err := d.clientset.CoreV1().Endpoints(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
	if err == nil && endpoints != nil {
		for _, subset := range endpoints.Subsets {
			if len(subset.Addresses) > 0 {
				hasEndpoints = true
				break
			}
		}
	}

	// Determine health
	health := models.HealthHealthy
	if !hasEndpoints && svc.Spec.Type != "ExternalName" {
		health = models.HealthDegraded
	}

	service := models.K8sService{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("service/%s/%s", svc.Namespace, svc.Name),
			Type:        models.EntityTypeK8sService,
			Labels:      svc.Labels,
			Annotations: svc.Annotations,
			Health:      health,
			Timestamp:   time.Now(),
		},
		Name:         svc.Name,
		Namespace:    svc.Namespace,
		ServiceType:  string(svc.Spec.Type),
		ClusterIP:    svc.Spec.ClusterIP,
		ExternalIPs:  svc.Spec.ExternalIPs,
		Ports:        ports,
		Selector:     svc.Spec.Selector,
		HasEndpoints: hasEndpoints,
	}

	return service
}

// GetIngresses collects all ingresses in the specified namespace
func (d *Discovery) GetIngresses(ctx context.Context, namespace string) ([]models.Ingress, error) {
	ingressList, err := d.clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ingresses: %w", err)
	}

	ingresses := make([]models.Ingress, 0, len(ingressList.Items))
	for _, ing := range ingressList.Items {
		ingress := d.parseIngress(&ing)
		ingresses = append(ingresses, ingress)
	}

	return ingresses, nil
}

func (d *Discovery) parseIngress(ing *networkingv1.Ingress) models.Ingress {
	// Parse rules
	rules := make([]models.IngressRule, 0, len(ing.Spec.Rules))
	for _, r := range ing.Spec.Rules {
		paths := make([]models.IngressPath, 0)
		if r.HTTP != nil {
			for _, p := range r.HTTP.Paths {
				pathType := "Prefix"
				if p.PathType != nil {
					pathType = string(*p.PathType)
				}

				serviceName := ""
				servicePort := int32(0)
				if p.Backend.Service != nil {
					serviceName = p.Backend.Service.Name
					if p.Backend.Service.Port.Number != 0 {
						servicePort = p.Backend.Service.Port.Number
					} else if p.Backend.Service.Port.Name != "" {
						// Try to parse port name as number
						if port, err := strconv.Atoi(p.Backend.Service.Port.Name); err == nil {
							servicePort = int32(port)
						}
					}
				}

				paths = append(paths, models.IngressPath{
					Path:        p.Path,
					PathType:    pathType,
					ServiceName: serviceName,
					ServicePort: servicePort,
				})
			}
		}

		rules = append(rules, models.IngressRule{
			Host:  r.Host,
			Paths: paths,
		})
	}

	// Parse TLS
	tls := make([]models.IngressTLS, 0, len(ing.Spec.TLS))
	for _, t := range ing.Spec.TLS {
		tls = append(tls, models.IngressTLS{
			Hosts:      t.Hosts,
			SecretName: t.SecretName,
		})
	}

	ingressClass := ""
	if ing.Spec.IngressClassName != nil {
		ingressClass = *ing.Spec.IngressClassName
	}

	ingress := models.Ingress{
		BaseEntity: models.BaseEntity{
			ID:          fmt.Sprintf("ingress/%s/%s", ing.Namespace, ing.Name),
			Type:        models.EntityTypeIngress,
			Labels:      ing.Labels,
			Annotations: ing.Annotations,
			Health:      models.HealthHealthy,
			Timestamp:   time.Now(),
		},
		Name:         ing.Name,
		Namespace:    ing.Namespace,
		IngressClass: ingressClass,
		Rules:        rules,
		TLS:          tls,
	}

	return ingress
}
