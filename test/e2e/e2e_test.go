/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/plumber-cd/argocd-autoscaler/test/utils"
)

// namespace where the project is deployed in
const namespace = "argocd-autoscaler"

// serviceAccountName created for the project
const serviceAccountName = "argocd-autoscaler-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "argocd-autoscaler-controller-manager-metrics-service"

const shardName = "argocd-autoscaler-sample-cluster"
const shardManagerName = "argocd-autoscaler-secrettypeclustershardmanager-sample"
const pollerName = "argocd-autoscaler-prometheuspoll-sample"
const normalizerName = "argocd-autoscaler-robustscalingnormalizer-sample"
const loadIndexerName = "argocd-autoscaler-weightedpnormloadindex-sample"
const partitionName = "argocd-autoscaler-longestprocessingtimepartition-sample"
const evaluationName = "argocd-autoscaler-mostwantedtwophasehysteresisevaluation-sample"
const scalingName = "argocd-autoscaler-replicasetscaler-sample"
const stsName = "argocd-autoscaler-argocd-application-controller"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string
	var deployLogs string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("labeling the namespace for prometheus")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"prometheus=argocd-autoscaler")
		_, err = Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("labeling the namespace for metrics")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"metrics=enabled")
		_, err = Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("creating sample resources")
		cmd = exec.Command("kubectl", "apply", "-k", "config/e2e/samples")
		_, err = Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create sample resources")

		By("deploying the controller-manager")
		// This actually has to do with cert manager struggling to inject CAs into CRDs
		Eventually(func(g Gomega) {
			cmd = exec.Command("make", "deploy-e2e")
			deployLogs, err = Run(cmd)
			g.Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
		}, "60s").Should(Succeed())
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy-e2e")
		_, _ = Run(cmd)

		By("undeploying sample resources")
		cmd = exec.Command("kubectl", "delete", "--ignore-not-found=true", "-k", "config/e2e/samples")
		_, _ = Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching deploy logs")
			_, _ = fmt.Fprintf(GinkgoWriter, "Deploy logs:\n%s", deployLogs)

			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n%s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n%s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(5 * time.Minute)
	SetDefaultEventuallyPollingInterval(5 * time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "app.kubernetes.io/name=argocd-autoscaler",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("validating that the metrics service is available")
			cmd := exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("validating that the ServiceMonitor for Prometheus is applied in the namespace")
			cmd = exec.Command("kubectl", "get", "ServiceMonitor", "-n", namespace)
			_, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "ServiceMonitor should exist")

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\tServing metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())
		})

		It("should ensure controller reconciled resources", func() {
			By("validating that the shard manager exported shards")
			verifyShardsDiscovered := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"-n", namespace,
					"secrets",
					"-l", "test.argocd.argoproj.io/secret-type=cluster",
					"-o", "go-template={{ range .items }}"+
						"{{ .metadata.uid }}"+
						"{{ end }}",
				)
				podOutput, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve secrets with type cluster")
				secretUIDs := GetNonEmptyLines(podOutput)
				g.Expect(secretUIDs).To(HaveLen(1), "expected 1 shard")

				cmd = exec.Command("kubectl", "get",
					"-n", namespace,
					"secrettypeclustershardmanagers.autoscaler.argoproj.io",
					shardManagerName,
					"-o", "go-template={{ range .status.shards }}"+
						"{{ .uid }}"+
						"{{ end }}",
				)
				podOutput, err = Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve shard manager")
				shardsUIDs := GetNonEmptyLines(podOutput)
				g.Expect(shardsUIDs).To(HaveLen(len(secretUIDs)), "expected all secrets to be discovered as shards")
			}
			Eventually(verifyShardsDiscovered).Should(Succeed())

			By("validating that the poller exported metrics")
			verifyPoller := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"-n", namespace,
					"prometheuspolls.autoscaler.argoproj.io",
					pollerName,
					"-o", "go-template={{ len .status.values }}",
				)
				discoveredMetrics, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve poller information")
				g.Expect(discoveredMetrics).To(Equal("1"), "expected to poll number of shards * number of metrics")
			}
			Eventually(verifyPoller).Should(Succeed())

			By("validating that the normalizer exported metrics")
			verifyNormalizer := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"-n", namespace,
					"robustscalingnormalizers.autoscaler.argoproj.io",
					normalizerName,
					"-o", "go-template={{ len .status.values }}",
				)
				normalizedMetrics, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve normalizer information")
				g.Expect(normalizedMetrics).To(Equal("1"), "expected to normalize all polled metrics")
			}
			Eventually(verifyNormalizer).Should(Succeed())

			By("validating that the load indexer calculated successfully")
			verifyLoadIndexer := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"-n", namespace,
					"weightedpnormloadindexes.autoscaler.argoproj.io",
					loadIndexerName,
					"-o", "go-template={{ len .status.values }}",
				)
				loadIndexes, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve load indexer information")
				g.Expect(loadIndexes).To(Equal("1"), "expected number of load indexes equal to the number of shards")
			}
			Eventually(verifyLoadIndexer).Should(Succeed())

			By("validating that partition was successful")
			verifyPartition := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"-n", namespace,
					"longestprocessingtimepartitions.autoscaler.argoproj.io",
					partitionName,
					"-o", "go-template={{ len .status.replicas }}",
				)
				partitions, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve partition information")
				g.Expect(partitions).To(Equal("1"), "expected number of partitions equal to the number of shards")
			}
			Eventually(verifyPartition).Should(Succeed())

			By("validating that evaluation was successful")
			verifyEvaluation := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"-n", namespace,
					"mostwantedtwophasehysteresisevaluations.autoscaler.argoproj.io",
					evaluationName,
					"-o", "go-template={{ len .status.replicas }}",
				)
				partitions, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve partition information")
				g.Expect(partitions).To(Equal("1"), "expected number of partitions equal to the number of shards")
			}
			Eventually(verifyEvaluation).Should(Succeed())

			By("validating that the scaling attempt was successful")
			verifyScaling := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"-n", namespace,
					"replicasetscalers.autoscaler.argoproj.io",
					scalingName,
					"-o", "go-template="+
						"{{ range .status.replicas }}"+
						"{{ $id := .id }}"+
						"{{ range .loadIndexes }}"+
						"{{ $id }}:{{ .shard.id }}"+
						"{{ end }}"+
						"{{ end }}",
				)
				podOutput, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve scaler information")
				partitions := GetNonEmptyLines(podOutput)
				g.Expect(partitions).To(HaveLen(1), "expected 1 partition")

				cmd = exec.Command("kubectl", "get",
					"-n", namespace,
					"secrets",
					shardName,
					"-o", "go-template={{ .data.shard }}",
				)
				partitionIndex, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve partition information")
				g.Expect(partitionIndex).To(Equal("MA=="), "expected shard 1 to be assigned to replica 1")
			}
			Eventually(verifyScaling).Should(Succeed())

			By("validating that the scaling was actually successful")
			verifyAppControllersScaled := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "app.kubernetes.io/name=argocd-application-controller",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve application controller pods information")
				podNames := GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 argocd-application-controller pods running")
				for i, podName := range podNames {
					g.Expect(podName).
						To(ContainSubstring(fmt.Sprintf("%s-%s", stsName, fmt.Sprint(i))))

					// Validate the pod's status
					cmd = exec.Command("kubectl", "get",
						"pods", podName, "-o", "jsonpath={.status.phase}",
						"-n", namespace,
					)
					output, err := Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("Running"), "Incorrect app controller pod status "+podName)
				}
			}
			Eventually(verifyAppControllersScaled).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")
			createCurlMetricsPod := func(g Gomega) {
				cmd := exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
					"--namespace", namespace,
					"--image=curlimages/curl:latest",
					"--overrides",
					fmt.Sprintf(`{
                    "metadata": {
                        "labels": {
                            "metrics": "enabled"
                        }
                    },
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": ["`+
						`while true; `+
						`do `+
						`URL=\"https://%s.%s.svc.cluster.local:8443/metrics\"; `+
						`TOKEN=\"$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)\"; `+
						`curl --fail --connect-timeout 5 --silent -L `+
						`-k -H \"Authorization: Bearer $TOKEN\" `+
						`\"$URL\" && `+
						`exit 0; `+
						`sleep 5s; `+
						`done`+
						`"],
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccount": "%s"
					}
				}`, metricsServiceName, namespace, serviceAccountName))
				_, err := Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")
			}
			Eventually(createCurlMetricsPod).Should(Succeed())

			By("waiting for the curl-metrics pod to complete.")
			waitCurlMetricsPod := func(g Gomega) {
				verifyCurlUp := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
						"-o", "jsonpath={.status.phase}",
						"-n", namespace)
					output, err := Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
				}
				Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

				By("getting the metrics by checking curl-metrics logs")
				metricsOutput := getMetricsOutput()
				Expect(metricsOutput).To(ContainSubstring(
					"controller_runtime_reconcile_total",
				))
			}
			Eventually(waitCurlMetricsPod).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			func() {
				metricsOutput := getMetricsOutput()

				expectedMetrics := ExpectedPrometheusMetricsMap(
					CreatePrometheusMetric("argocd_autoscaler_shard_manager_discovered_shards",
						map[string]string{
							"shard_manager_type": "secret_type_cluster",
							"shard_manager_ref":  fmt.Sprintf("%s/%s", namespace, shardManagerName),
							"shard_uid":          "*",
							"shard_id":           shardName,
							"shard_namespace":    namespace,
							"shard_name":         "sample-cluster",
							"shard_server":       "https://kubernetes.default.svc",
							"replica_id":         "0",
						}, 1),

					CreatePrometheusMetric("argocd_autoscaler_poll_values",
						map[string]string{
							"poll_type":       "prometheus",
							"poll_ref":        fmt.Sprintf("%s/%s", namespace, pollerName),
							"shard_uid":       "*",
							"shard_id":        shardName,
							"shard_namespace": namespace,
							"shard_name":      "sample-cluster",
							"shard_server":    "https://kubernetes.default.svc",
							"metric_id":       "up",
						}, 1),

					CreatePrometheusMetric("argocd_autoscaler_normalizer_values",
						map[string]string{
							"normalizer_type": "robust_scaling",
							"normalizer_ref":  fmt.Sprintf("%s/%s", namespace, normalizerName),
							"shard_uid":       "*",
							"shard_id":        shardName,
							"shard_namespace": namespace,
							"shard_name":      "sample-cluster",
							"shard_server":    "https://kubernetes.default.svc",
							"metric_id":       "up",
						}, 1),

					CreatePrometheusMetric("argocd_autoscaler_load_index_values",
						map[string]string{
							"load_index_type": "weighted_p_norm",
							"load_index_ref":  fmt.Sprintf("%s/%s", namespace, loadIndexerName),
							"shard_uid":       "*",
							"shard_id":        shardName,
							"shard_namespace": namespace,
							"shard_name":      "sample-cluster",
							"shard_server":    "https://kubernetes.default.svc",
						}, 1),

					CreatePrometheusMetric("argocd_autoscaler_partition_shards",
						map[string]string{
							"partition_type":  "longest_processing_time",
							"partition_ref":   fmt.Sprintf("%s/%s", namespace, partitionName),
							"shard_uid":       "*",
							"shard_id":        shardName,
							"shard_namespace": namespace,
							"shard_name":      "sample-cluster",
							"shard_server":    "https://kubernetes.default.svc",
							"replica_id":      "0",
						}, 1),

					CreatePrometheusMetric("argocd_autoscaler_partition_replicas_total_load",
						map[string]string{
							"partition_type": "longest_processing_time",
							"partition_ref":  fmt.Sprintf("%s/%s", namespace, partitionName),
							"replica_id":     "0",
						}, 1),

					CreatePrometheusMetric("argocd_autoscaler_evaluation_projected_shards",
						map[string]string{
							"evaluation_type": "most_wanted_two_phase_hysteresis",
							"evaluation_ref":  fmt.Sprintf("%s/%s", namespace, evaluationName),
							"shard_uid":       "*",
							"shard_id":        shardName,
							"shard_namespace": namespace,
							"shard_name":      "sample-cluster",
							"shard_server":    "https://kubernetes.default.svc",
							"replica_id":      "0",
						}, 1),

					CreatePrometheusMetric("argocd_autoscaler_evaluation_projected_replicas_total_load",
						map[string]string{
							"evaluation_type": "most_wanted_two_phase_hysteresis",
							"evaluation_ref":  fmt.Sprintf("%s/%s", namespace, evaluationName),
							"replica_id":      "0",
						}, 1),

					CreatePrometheusMetric("argocd_autoscaler_evaluation_shards",
						map[string]string{
							"evaluation_type": "most_wanted_two_phase_hysteresis",
							"evaluation_ref":  fmt.Sprintf("%s/%s", namespace, evaluationName),
							"shard_uid":       "*",
							"shard_id":        shardName,
							"shard_namespace": namespace,
							"shard_name":      "sample-cluster",
							"shard_server":    "https://kubernetes.default.svc",
							"replica_id":      "0",
						}, 1),

					CreatePrometheusMetric("argocd_autoscaler_evaluation_replicas_total_load",
						map[string]string{
							"evaluation_type": "most_wanted_two_phase_hysteresis",
							"evaluation_ref":  fmt.Sprintf("%s/%s", namespace, evaluationName),
							"replica_id":      "0",
						}, 1),

					CreatePrometheusMetric("argocd_autoscaler_scaler_replica_set_changes_total",
						map[string]string{
							"scaler_type":                 "replica_set",
							"scaler_ref":                  fmt.Sprintf("%s/%s", namespace, scalingName),
							"replica_set_controller_kind": "StatefulSet",
							"replica_set_controller_ref":  fmt.Sprintf("%s/%s", namespace, stsName),
						}, 1),
				)

				Expect(metricsOutput).To(MatchPrometheusMetrics(expectedMetrics))
			}()
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks

	})
})

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(HavePrefix("# HELP"))
	return metricsOutput
}
