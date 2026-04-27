import { redirect } from "next/navigation";

type Props = {
  params: {
    token: string;
  };
};

export default function ShareShortRoute({ params }: Props) {
  redirect(`/public/share/${encodeURIComponent(params.token)}`);
}

