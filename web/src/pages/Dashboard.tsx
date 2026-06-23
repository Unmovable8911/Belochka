import { AddServerDialog } from "@/components/AddServerDialog"

export default function Dashboard() {
  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <AddServerDialog />
      </div>
      <p className="text-muted-foreground">Server overview will appear here.</p>
    </div>
  )
}
