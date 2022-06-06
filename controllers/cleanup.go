package controllers

import (
	"context"
	"fmt"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	datamoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var cleanupVSBTypes = []client.Object{
	&corev1.PersistentVolumeClaim{},
	&corev1.Pod{},
	&snapv1.VolumeSnapshot{},
	&snapv1.VolumeSnapshotContent{},
	&corev1.Secret{},
	&volsyncv1alpha1.ReplicationSource{},
}

func (r *VolumeSnapshotBackupReconciler) CleanVSBBackupResources(log logr.Logger) (bool, error) {
	r.Log.Info("In function CleanBackupResources")
	// get volumesnapshotbackup from cluster
	vsb := datamoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		return false, err
	}

	// make sure VSB is completed before deleting resources
	if vsb.Status.Phase != datamoverv1alpha1.DatamoverVolSyncPhaseCompleted {
		r.Log.Info("waiting for volSync to complete before deleting resources")
		return false, nil
	}

	// get resources with VSB controller label in protected ns
	deleteOptions := []client.DeleteAllOfOption{
		client.MatchingLabels{VSBLabel: vsb.Name},
		client.InNamespace(vsb.Spec.ProtectedNamespace),
	}

	for _, obj := range cleanupVSBTypes {
		err := r.DeleteAllOf(r.Context, obj, deleteOptions...)
		if err != nil {
			r.Log.Error(err, "unable to delete VSB resource")
			return false, err
		}
	}

	// check resources have been deleted
	// resourcesDeleted, err := r.areVSBResourcesDeleted(r.Log, &vsb)
	// if err != nil || !resourcesDeleted {
	// 	r.Log.Error(err, "not all VSB resources have been deleted")
	// 	return false, err
	// }

	// Update VSB status as completed
	vsb.Status.Phase = datamoverv1alpha1.DatamoverBackupPhaseCompleted
	err := r.Status().Update(context.Background(), &vsb)
	if err != nil {
		return false, err
	}
	r.Log.Info("returning from cleaning VSB resources as completed")
	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) areVSBResourcesDeleted(log logr.Logger, vsb *datamoverv1alpha1.VolumeSnapshotBackup) (bool, error) {

	// check the cloned PVC has been deleted
	clonedPVC := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Name: fmt.Sprintf("%s-pvc", vsb.Spec.VolumeSnapshotContent.Name), Namespace: vsb.Spec.ProtectedNamespace}, &clonedPVC); err != nil {

		// we expect resource to not be found
		if k8serror.IsNotFound(err) {
			r.Log.Info("cloned volumesnapshot has been deleted")
		}
		// other error
		return false, err
	}

	// check dummy pod is deleted
	dummyPod := corev1.Pod{}
	if err := r.Get(r.Context, types.NamespacedName{Name: fmt.Sprintf("%s-pod", vsb.Name), Namespace: vsb.Spec.ProtectedNamespace}, &dummyPod); err != nil {

		// we expect resource to not be found
		if k8serror.IsNotFound(err) {
			r.Log.Info("dummy pod has been deleted")
		}
		// other error
		return false, err
	}

	// check the cloned VSC has been deleted
	vscClone := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: fmt.Sprintf("%s-clone", vsb.Spec.VolumeSnapshotContent.Name)}, &vscClone); err != nil {

		// we expect resource to not be found
		if k8serror.IsNotFound(err) {
			r.Log.Info("cloned volumesnapshotcontent has been deleted")
		}
		// other error
		return false, err
	}

	// check the cloned VS has been deleted
	vsClone := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: fmt.Sprintf(vscClone.Spec.VolumeSnapshotRef.Name), Namespace: vsb.Spec.ProtectedNamespace}, &vsClone); err != nil {

		// we expect resource to not be found
		if k8serror.IsNotFound(err) {
			r.Log.Info("cloned volumesnapshot has been deleted")
		}
		// other error
		return false, err
	}

	// check secret has been deleted
	secret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Name: fmt.Sprintf("%s-secret", vsb.Name), Namespace: vsb.Spec.ProtectedNamespace}, &secret); err != nil {

		// we expect resource to not be found
		if k8serror.IsNotFound(err) {
			r.Log.Info("restic secret has been deleted")
		}
		// other error
		return false, err
	}

	// check replicationSource has been deleted
	repSource := volsyncv1alpha1.ReplicationSource{}
	if err := r.Get(r.Context, types.NamespacedName{Name: fmt.Sprintf("%s-rep-src", vsb.Name), Namespace: vsb.Spec.ProtectedNamespace}, &repSource); err != nil {

		// we expect resource to not be found
		if k8serror.IsNotFound(err) {
			r.Log.Info("replicationSource has been deleted")
		}
		// other error
		return false, err
	}

	//all resources have been deleted
	r.Log.Info("all VSB resources have been deleted")
	return true, nil
}
