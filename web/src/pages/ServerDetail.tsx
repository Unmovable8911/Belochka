import { useParams } from "react-router-dom"

export default function ServerDetail() {
  const { id } = useParams<{ id: string }>()

  return (
    <div className="p-6">
      <h1 className="text-2xl font-bold">Server Detail</h1>
      <p className="text-muted-foreground">Details for server {id} will appear here.</p>
    </div>
  )
}
