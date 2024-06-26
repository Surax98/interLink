package slurm

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/containerd/containerd/log"

	commonIL "github.com/intertwin-eu/interlink/pkg/common"
)

// SubmitHandler generates and submits a SLURM batch script according to provided data.
// 1 Pod = 1 Job. If a Pod has multiple containers, every container is a line with it's parameters in the SLURM script.
func (h *SidecarHandler) SubmitHandler(w http.ResponseWriter, r *http.Request) {
	log.G(h.Ctx).Info("Slurm Sidecar: received Submit call")
	statusCode := http.StatusOK
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		statusCode = http.StatusInternalServerError
		w.WriteHeader(statusCode)
		w.Write([]byte("Some errors occurred while creating container. Check Slurm Sidecar's logs"))
		log.G(h.Ctx).Error(err)
		return
	}

	var req []commonIL.RetrievedPodData
	err = json.Unmarshal(bodyBytes, &req)
	if err != nil {
		statusCode = http.StatusInternalServerError
		w.WriteHeader(statusCode)
		w.Write([]byte("Some errors occurred while creating container. Check Slurm Sidecar's logs"))
		log.G(h.Ctx).Error(err)
		return
	}

	for _, test := range req {
		log.G(h.Ctx).Debug(test.Pod.UID)
	}

	for _, data := range req {
		containers := data.Pod.Spec.Containers
		metadata := data.Pod.ObjectMeta
		filesPath := h.Config.DataRootFolder + data.Pod.Namespace + "-" + string(data.Pod.UID)

		var singularity_command_pod []SingularityCommand

		for _, container := range containers {
			log.G(h.Ctx).Info("- Beginning script generation for container " + container.Name)
			singularityPrefix := commonIL.InterLinkConfigInst.SingularityPrefix
			if singularityAnnotation, ok := metadata.Annotations["job.vk.io/singularity-commands"]; ok {
				singularityPrefix += " " + singularityAnnotation
			}

			//container.Resources.Requests
			//overlayPath := h.Config.DataRootFolder + data.Pod.Namespace + "-" + string(data.Pod.UID) + "/overlay.img"
			createOverlay := []string{}
			//createOverlay := []string{"singularity", "overlay", "create", "--sparse", "--size=1024", overlayPath, "&&"}

			commstr1 := []string{"singularity", "exec", "--nv"}
			//commstr1 := []string{"singularity", "exec", "--containall", "--overlay", overlayPath, "--nv"}
			//	h.Config.DataRootFolder + string(data.Pod.UID) }

			envs := prepareEnvs(h.Ctx, container)
			image := ""
			mounts, err := prepareMounts(h.Ctx, h.Config, req, container, filesPath)
			log.G(h.Ctx).Debug(mounts)
			if err != nil {
				statusCode = http.StatusInternalServerError
				w.WriteHeader(statusCode)
				w.Write([]byte("Error prepairing mounts. Check Slurm Sidecar's logs"))
				log.G(h.Ctx).Error(err)
				os.RemoveAll(filesPath)
				return
			}

			image = container.Image
			if strings.HasPrefix(container.Image, "/") {
				if image_uri, ok := metadata.Annotations["slurm-job.vk.io/image-root"]; ok {
					image = image_uri + container.Image
				} else {
					log.G(h.Ctx).Info("- image-uri annotation not specified for path in remote filesystem")
				}
			} else {
				image = container.Image
			}

			log.G(h.Ctx).Debug("-- Appending all commands together...")
			singularity_command := append(createOverlay, commstr1...)
			singularity_command = append(singularity_command, envs...)
			singularity_command = append(singularity_command, mounts...)
			singularity_command = append(singularity_command, image)
			singularity_command = append(singularity_command, container.Command...)
			singularity_command = append(singularity_command, container.Args...)

			singularity_command_pod = append(singularity_command_pod, SingularityCommand{command: singularity_command, containerName: container.Name})
		}

		path, err := produceSLURMScript(h.Ctx, h.Config, data.Pod.Namespace, string(data.Pod.UID), filesPath, metadata, singularity_command_pod)
		if err != nil {
			statusCode = http.StatusInternalServerError
			w.WriteHeader(statusCode)
			w.Write([]byte("Error producing Slurm script. Check Slurm Sidecar's logs"))
			log.G(h.Ctx).Error(err)
			os.RemoveAll(filesPath)
			return
		}
		out, err := SLURMBatchSubmit(h.Ctx, h.Config, path)
		if err != nil {
			statusCode = http.StatusInternalServerError
			w.WriteHeader(statusCode)
			w.Write([]byte("Error submitting Slurm script. Check Slurm Sidecar's logs"))
			log.G(h.Ctx).Error(err)
			os.RemoveAll(filesPath)
			return
		}
		log.G(h.Ctx).Info(out)
		err = handleJID(h.Ctx, data.Pod, string(data.Pod.UID), h.JIDs, out, filesPath)
		if err != nil {
			statusCode = http.StatusInternalServerError
			w.WriteHeader(statusCode)
			w.Write([]byte("Error handling JID. Check Slurm Sidecar's logs"))
			log.G(h.Ctx).Error(err)
			os.RemoveAll(filesPath)
			err = deleteContainer(h.Ctx, h.Config, string(data.Pod.UID), h.JIDs, filesPath)
			return
		}
	}

	w.WriteHeader(statusCode)

	if statusCode != http.StatusOK {
		w.Write([]byte("Some errors occurred while creating containers. Check Slurm Sidecar's logs"))
	} else {
		w.Write([]byte("Containers created"))
	}
}
