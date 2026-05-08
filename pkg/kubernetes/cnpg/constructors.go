package cnpg

import (
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	barmanv1 "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateCluster returns a new CNPG Cluster with TypeMeta and ObjectMeta set.
func CreateCluster(name, namespace string) *cnpgv1.Cluster {
	return &cnpgv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: cnpgv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateDatabase returns a new CNPG Database with TypeMeta and ObjectMeta set.
func CreateDatabase(name, namespace string) *cnpgv1.Database {
	return &cnpgv1.Database{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Database",
			APIVersion: cnpgv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateObjectStore returns a new CNPG ObjectStore with TypeMeta and ObjectMeta set.
func CreateObjectStore(name, namespace string) *barmanv1.ObjectStore {
	return &barmanv1.ObjectStore{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ObjectStore",
			APIVersion: barmanv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateScheduledBackup returns a new CNPG ScheduledBackup with TypeMeta and ObjectMeta set.
func CreateScheduledBackup(name, namespace string) *cnpgv1.ScheduledBackup {
	return &cnpgv1.ScheduledBackup{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ScheduledBackup",
			APIVersion: cnpgv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreatePooler returns a new CNPG Pooler with TypeMeta and ObjectMeta set.
func CreatePooler(name, namespace string) *cnpgv1.Pooler {
	return &cnpgv1.Pooler{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pooler",
			APIVersion: cnpgv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
