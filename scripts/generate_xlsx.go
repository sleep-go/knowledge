package main

import (
	"fmt"
	"github.com/xuri/excelize/v2"
	"log"
)

func main() {
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	// 创建一个工作表
	sheetName := "Sheet1"
	index, _ := f.NewSheet(sheetName)

	// 设置单元格的值
	f.SetCellValue(sheetName, "A1", "姓名")
	f.SetCellValue(sheetName, "B1", "职业")
	f.SetCellValue(sheetName, "C1", "描述")

	f.SetCellValue(sheetName, "A2", "张三")
	f.SetCellValue(sheetName, "B2", "软件工程师")
	f.SetCellValue(sheetName, "C2", "张三是一个精通 Go 语言和 AI 技术的专家，目前正在开发一个本地知识库系统。")

	f.SetCellValue(sheetName, "A3", "李四")
	f.SetCellValue(sheetName, "B3", "产品经理")
	f.SetCellValue(sheetName, "C3", "李四负责知识库系统的产品设计，他认为支持 Excel 文件是非常重要的功能。")

	// 设置默认工作表
	f.SetActiveSheet(index)

	// 保存文件
	if err := f.SaveAs("test_kb/test_data.xlsx"); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Excel 文件生成成功: test_kb/test_data.xlsx")
}
