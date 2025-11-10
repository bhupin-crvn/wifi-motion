package kubeutils

// func TestCheckMemoryAvailability(t *testing.T) {
// 	kconfig := KubernetesConfig{Clientset: fake.NewSimpleClientset()}

// 	bool, err := kconfig.CheckGpuAvaibility("1")
// 	fmt.Println("Say hi")

// 	if err == nil {
// 		t.Log(err)
// 	}
// 	if bool == true {
// 		t.Log(" Memory Found")
// 	}
// }

// func TestFakeClientSet(t *testing.T) {
// 	ctx := context.TODO()
// 	cs := fake.NewSimpleClientset()

// 	nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	for _, node := range nodes.Items {
// 		memoryQty := node.Status.Allocatable[corev1.ResourceMemory]
// 		memoryRequestQty := resource.MustParse("1Gi")
// 		if memoryQty.Cmp(memoryRequestQty) >= 0 {
// 			t.Error("ok")
// 		}
// 	}
// }

// func TestGetNodeResources(t *testing.T) {
// 	// Create a fake client with a node.
// 	client := fake.NewSimpleClientset(&corev1.Node{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name: "node1",
// 		},
// 		Status: corev1.NodeStatus{
// 			Capacity: corev1.ResourceList{
// 				"cpu":            resource.MustParse("1000m"),
// 				"memory":         resource.MustParse("1000Mi"),
// 				"nvidia.com/gpu": resource.MustParse("1"),
// 			},
// 		},
// 	})

// 	kc := &KubernetesConfig{
// 		Clientset: client,
// 	}

// 	resources, err := kc.GetNodeResources()
// 	if err != nil {
// 		t.Fatalf("expected no error, got %v", err)
// 	}

// 	if len(resources) != 1 {
// 		t.Fatalf("expected one node, got %d", len(resources))
// 	}

// 	nodeResources, ok := resources["node1"]
// 	if !ok {
// 		t.Fatalf("expected resources for node1, got none")
// 	}

// 	if nodeResources.CPU != "1" {
// 		t.Errorf("expected CPU to be 1, got %s", nodeResources.CPU)
// 	}

// 	if nodeResources.Memory != "1000Mi" {
// 		t.Errorf("expected Memory to be 1000Mi, got %s", nodeResources.Memory)
// 	}

// 	if nodeResources.GPU != "1" {
// 		t.Errorf("expected GPU to be 1, got %s", nodeResources.GPU)
// 	}
// }
