package kubeutils

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LogsOptions struct {
	Follow      bool
	Tail        int64
	Container   string
	Namespace   string
	PodName     string
	LogsOptions *apiv1.PodLogOptions
}

func NewLogsOptions() *LogsOptions {
	return &LogsOptions{
		Follow: false,
		Tail:   200,
	}
}

func (kc *KubernetesConfig) StreamDeploymentLog(deploymentName string, ctx context.Context, opts *LogsOptions, stream io.Writer) error {

	pods, err := kc.Clientset.CoreV1().Pods(opts.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=" + deploymentName,
	})
	if err != nil {
		return fmt.Errorf("error getting pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for deployment: %s", deploymentName)
	}
	podName := pods.Items[0].Name

	podLogOpts := &apiv1.PodLogOptions{
		Follow:    opts.Follow,
		Container: opts.Container,
	}

	if opts.Tail >= 0 {
		podLogOpts.TailLines = &opts.Tail
	}

	req := kc.Clientset.CoreV1().Pods(opts.Namespace).GetLogs(podName, podLogOpts)

	podLogs, err := req.Stream(ctx)

	if err != nil {
		return fmt.Errorf("error in opening stream: %w", err)
	}
	defer podLogs.Close()

	buf := bufio.NewReader(podLogs)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			line, err := buf.ReadBytes('\n')

			if err != nil {
				if err == io.EOF {
					return nil
				}
				return fmt.Errorf("error in reading log stream: %w", err)
			}

			_, err = stream.Write(line)
			if err != nil {
				return fmt.Errorf("error in writing log to stream: %w", err)
			}

			if flusher, ok := stream.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

func (kc *KubernetesConfig) GetDeploymentLog(deploymentName, namespace string, ctx context.Context, opts *LogsOptions, stream io.Writer) error {
	pods, err := kc.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", deploymentName),
	})
	if err != nil {
		return fmt.Errorf("error listing pods: %v", err)
	}

	for _, pod := range pods.Items {
		if err := kc.getPodLogs(namespace, pod.Name, ctx, opts, stream); err != nil {
			return err
		}
	}
	return nil
}

func (kc *KubernetesConfig) getPodLogs(namespace, podName string, ctx context.Context, opts *LogsOptions, stream io.Writer) error {
	req := kc.Clientset.CoreV1().Pods(namespace).GetLogs(podName, &apiv1.PodLogOptions{
		Follow: opts.Follow,
	})
	logStream, err := req.Stream(ctx)

	if err != nil {
		return fmt.Errorf("error opening stream for pod %s: %v", podName, err)
	}
	defer logStream.Close()

	_, err = io.Copy(stream, logStream)
	if err != nil {
		return fmt.Errorf("error reading log stream for pod %s: %v", podName, err)
	}

	return nil
}
