package main

import (
	"fmt"
	"github.com/egokernel/ek1/internal/profile"
)
func main(){
	progress := profile.EKProgress{Shadow: true, Hand: false, Voice: true}
	fmt.Println(progress)
	fmt.Println("Shadow", progress.Shadow)
	fmt.Println("Hand", progress.Hand)
}