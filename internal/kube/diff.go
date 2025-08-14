package kube

import (
	"clustershift/internal/exit"
	"fmt"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Clusters) CreateResourceDiff(resourceType ResourceType) {
	diffResources, err := c.getResourceDiff(resourceType)
	if err != nil {
		//fmt.Printf("Error getting %s diff: %v\n", resourceType, err)
		return
	}

	//fmt.Printf("%ss in original but not in target:\n", resourceType)
	/*
		for _, resource := range diffResources.([]interface{}) {
			meta := reflect.ValueOf(resource).FieldByName("ObjectMeta").Interface().(metav1.ObjectMeta)
			//fmt.Printf("Namespace: %s, Name: %s\n", meta.Namespace, meta.Name)
		}
	*/

	for _, resource := range diffResources.([]interface{}) {
		newResource := CleanResourceForCreation(resource)
		resourceValue := reflect.ValueOf(newResource).Elem()

		namespace := resourceValue.FieldByName("ObjectMeta").FieldByName("Namespace").String()

		if namespace != "clustershift" {
			err := c.Target.CreateResource(resourceType, namespace, newResource)
			exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to create %s in target cluster", resourceType))
		}
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

func CleanResourceForCreation(resource interface{}) interface{} {
	resourceValue := reflect.ValueOf(resource)
	if resourceValue.Kind() == reflect.Ptr {
		resourceValue = resourceValue.Elem()
	}

	// Create a new instance of the resource
	newResource := reflect.New(resourceValue.Type()).Elem()

	// Copy TypeMeta (ensure kind and apiVersion are set)
	typeMeta := resourceValue.FieldByName("TypeMeta")
	newTypeMeta := newResource.FieldByName("TypeMeta")
	if typeMeta.IsValid() && newTypeMeta.IsValid() {
		kind := typeMeta.FieldByName("Kind")
		apiVersion := typeMeta.FieldByName("APIVersion")
		if kind.IsValid() {
			newTypeMeta.FieldByName("Kind").Set(kind)
		}
		if apiVersion.IsValid() {
			newTypeMeta.FieldByName("APIVersion").Set(apiVersion)
		}
	}

	// Copy and clean ObjectMeta
	objectMeta := resourceValue.FieldByName("ObjectMeta")
	newObjectMeta := newResource.FieldByName("ObjectMeta")
	if objectMeta.IsValid() && newObjectMeta.IsValid() {
		// Copy only the essential fields
		fieldsToKeep := []string{"Name", "Namespace", "Labels", "Annotations"}
		for _, field := range fieldsToKeep {
			value := objectMeta.FieldByName(field)
			if value.IsValid() {
				newObjectMeta.FieldByName(field).Set(value)
			}
		}
	}

	// Copy the spec/data if it exists (for different resource types)
	for _, field := range []string{"Data", "StringData", "Spec"} {
		sourceField := resourceValue.FieldByName(field)
		targetField := newResource.FieldByName(field)
		if sourceField.IsValid() && targetField.IsValid() {
			targetField.Set(sourceField)
		}
	}

	// Copy RoleRef and Subjects for ClusterRoleBinding
	if resourceValue.Type().Name() == "ClusterRoleBinding" {
		roleRef := resourceValue.FieldByName("RoleRef")
		newRoleRef := newResource.FieldByName("RoleRef")
		if roleRef.IsValid() && newRoleRef.IsValid() {
			newRoleRef.Set(roleRef)
		}
		subjects := resourceValue.FieldByName("Subjects")
		newSubjects := newResource.FieldByName("Subjects")
		if subjects.IsValid() && newSubjects.IsValid() {
			newSubjects.Set(subjects)
		}
	}

	return newResource.Addr().Interface()
}
