package kube

import (
	"fmt"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Clusters) CreateResourceDiff(resourceType ResourceType) {
	diffResources, err := c.getResourceDiff(resourceType)
	if err != nil {
		fmt.Printf("Error getting %s diff: %v\n", resourceType, err)
		return
	}

	fmt.Printf("%ss in original but not in target:\n", resourceType)
	for _, resource := range diffResources.([]interface{}) {
		meta := reflect.ValueOf(resource).FieldByName("ObjectMeta").Interface().(metav1.ObjectMeta)
		fmt.Printf("Namespace: %s, Name: %s\n", meta.Namespace, meta.Name)
	}

	for _, resource := range diffResources.([]interface{}) {
		newResource := cleanResourceForCreation(resource)
		resourceValue := reflect.ValueOf(newResource).Elem()
		namespace := resourceValue.FieldByName("ObjectMeta").FieldByName("Namespace").String()
		name := resourceValue.FieldByName("ObjectMeta").FieldByName("Name").String()

		var createErr error
		createErr = c.Target.CreateResource(resourceType, name, namespace, newResource)

		if createErr != nil {
			fmt.Printf("Error creating %s %s in namespace %s: %v\n", resourceType, name, namespace, createErr)
			continue
		}
		fmt.Printf("Successfully created %s %s in namespace %s\n", resourceType, name, namespace)
	}
}

func (c Clusters) getResourceDiff(resourceType ResourceType) (interface{}, error) {
	originalResources, err := c.Origin.FetchResources(resourceType)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s of origin cluster: %w", resourceType, err)
	}
	targetResources, err := c.Target.FetchResources(resourceType)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s of target cluster: %w", resourceType, err)
	}

	originalItems := reflect.ValueOf(originalResources).Elem().FieldByName("Items")
	targetItems := reflect.ValueOf(targetResources).Elem().FieldByName("Items")

	targetResourceMap := make(map[string]bool)
	for i := 0; i < targetItems.Len(); i++ {
		item := targetItems.Index(i).Interface()
		meta := reflect.ValueOf(item).FieldByName("ObjectMeta").Interface().(metav1.ObjectMeta)
		key := fmt.Sprintf("%s/%s", meta.Namespace, meta.Name)
		targetResourceMap[key] = true
	}

	var diffResources []interface{}
	for i := 0; i < originalItems.Len(); i++ {
		item := originalItems.Index(i).Interface()
		meta := reflect.ValueOf(item).FieldByName("ObjectMeta").Interface().(metav1.ObjectMeta)
		key := fmt.Sprintf("%s/%s", meta.Namespace, meta.Name)
		if !targetResourceMap[key] {
			diffResources = append(diffResources, item)
		}
	}

	return diffResources, nil
}

func cleanResourceForCreation(resource interface{}) interface{} {
	resourceValue := reflect.ValueOf(resource)
	if resourceValue.Kind() == reflect.Ptr {
		resourceValue = resourceValue.Elem()
	}

	// Create a new instance of the resource
	newResource := reflect.New(resourceValue.Type()).Elem()

	// Copy the relevant fields from the original resource
	newResource.FieldByName("ObjectMeta").Set(resourceValue.FieldByName("ObjectMeta"))
	newResource.FieldByName("TypeMeta").Set(resourceValue.FieldByName("TypeMeta"))

	objectMetaField := newResource.FieldByName("ObjectMeta")
	objectMetaField.FieldByName("ResourceVersion").SetString("")

	return newResource.Addr().Interface()
}
