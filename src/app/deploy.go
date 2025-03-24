package app

func (app *App) DeployBpmn() {
	app.zeebe.DeployWorkflow(app.config.BpmnPath)
}
